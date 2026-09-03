package learn

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	pidPrefixRegex      = regexp.MustCompile(`^\s*(?:\[pid\s+(\d+)\]|(\d+))\s+`)
	syscallCallRegex    = regexp.MustCompile(`^([a-zA-Z0-9_]+)\((.*)\)\s*=\s*(-?[0-9]+|0x[0-9a-fA-F]+|\?)(?:\s+(.*))?$`)
	unfinishedCallRegex = regexp.MustCompile(`^([a-zA-Z0-9_]+)\((.*)\s+<unfinished \.\.\.>$`)
	resumedCallRegex    = regexp.MustCompile(`^<\.\.\.\s+([a-zA-Z0-9_]+)\s+resumed>(.*)\)\s*=\s*(-?[0-9]+|0x[0-9a-fA-F]+|\?)(?:\s+(.*))?$`)
)

// PendingSyscall holds partial state for an unfinished strace syscall.
type PendingSyscall struct {
	Name string
	Args string
}

// TraceParser parses strace logs, handling multi-threaded/unfinished syscall reassembly.
type TraceParser struct {
	pending map[int]*PendingSyscall
}

// NewTraceParser creates a new stateful strace parser.
func NewTraceParser() *TraceParser {
	return &TraceParser{
		pending: make(map[int]*PendingSyscall),
	}
}

// ParseLine parses a single log line, reassembling unfinished calls when they resume.
func (p *TraceParser) ParseLine(line string) *ParsedSyscall {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "+++") || strings.HasPrefix(trimmed, "---") {
		return nil
	}

	pid := 0
	if match := pidPrefixRegex.FindStringSubmatch(trimmed); match != nil {
		pStr := match[1]
		if pStr == "" {
			pStr = match[2]
		}
		if parsedPID, err := strconv.Atoi(pStr); err == nil {
			pid = parsedPID
		}
		trimmed = strings.TrimSpace(trimmed[len(match[0]):])
	}

	// Check if this is an unfinished syscall
	if m := unfinishedCallRegex.FindStringSubmatch(trimmed); m != nil {
		p.pending[pid] = &PendingSyscall{
			Name: m[1],
			Args: m[2],
		}
		return nil
	}

	// Check if this is a resumed syscall
	if m := resumedCallRegex.FindStringSubmatch(trimmed); m != nil {
		callName := m[1]
		extraArgs := m[2]
		retStr := m[3]
		retErr := m[4]

		if pendingCall, ok := p.pending[pid]; ok && pendingCall.Name == callName {
			delete(p.pending, pid)
			completeArgs := pendingCall.Args + extraArgs
			retVal, _ := strconv.Atoi(retStr)
			success := retVal >= 0 && !strings.Contains(retErr, "ENOENT")

			parsed := &ParsedSyscall{
				PID:     pid,
				Name:    callName,
				RawArgs: completeArgs,
				RetVal:  retVal,
				Success: success,
			}
			populateParsedSyscall(parsed, callName, completeArgs)
			return parsed
		}
		return nil
	}

	if strings.HasPrefix(trimmed, "<...") {
		return nil
	}

	m := syscallCallRegex.FindStringSubmatch(trimmed)
	if m == nil {
		return nil
	}

	callName := m[1]
	argsStr := m[2]
	retStr := m[3]
	retErr := m[4]

	retVal := -1
	if parsedRet, err := strconv.Atoi(retStr); err == nil {
		retVal = parsedRet
	}
	success := retVal >= 0 && !strings.Contains(retErr, "ENOENT")

	parsed := &ParsedSyscall{
		PID:     pid,
		Name:    callName,
		RawArgs: argsStr,
		RetVal:  retVal,
		Success: success,
	}
	populateParsedSyscall(parsed, callName, argsStr)
	return parsed
}

func populateParsedSyscall(parsed *ParsedSyscall, callName, argsStr string) {
	switch callName {
	case "open", "openat", "openat2", "creat":
		paths := extractQuotedStrings(argsStr)
		if len(paths) > 0 {
			parsed.Paths = []string{paths[0]}
		}
		parsed.Mode = determineOpenMode(callName, argsStr)

	case "unlink", "unlinkat":
		paths := extractQuotedStrings(argsStr)
		if len(paths) > 0 {
			parsed.Paths = []string{paths[0]}
		}
		parsed.Mode = AccessWrite

	case "rename", "renameat", "renameat2":
		paths := extractQuotedStrings(argsStr)
		if len(paths) >= 2 {
			parsed.Paths = []string{paths[0], paths[1]}
		} else if len(paths) == 1 {
			parsed.Paths = []string{paths[0]}
		}
		parsed.Mode = AccessWrite

	case "mkdir", "mkdirat":
		paths := extractQuotedStrings(argsStr)
		if len(paths) > 0 {
			parsed.Paths = []string{paths[0]}
		}
		parsed.Mode = AccessWrite

	case "stat", "lstat", "newfstatat", "statx", "access", "faccessat", "faccessat2", "readlink", "readlinkat":
		paths := extractQuotedStrings(argsStr)
		if len(paths) > 0 {
			parsed.Paths = []string{paths[0]}
		}
		parsed.Mode = AccessRead

	case "truncate", "ftruncate":
		paths := extractQuotedStrings(argsStr)
		if len(paths) > 0 {
			parsed.Paths = []string{paths[0]}
		}
		parsed.Mode = AccessWrite

	case "connect", "bind":
		parsed.SockAddr = extractSockAddr(argsStr)
		parsed.Mode = AccessNone
	}
}

// ParseTraceLine parses a single line from an strace output log.
func ParseTraceLine(line string) *ParsedSyscall {
	p := NewTraceParser()
	return p.ParseLine(line)
}

func determineOpenMode(callName, argsStr string) AccessMode {
	if callName == "creat" {
		return AccessWrite
	}
	upper := strings.ToUpper(argsStr)
	if strings.Contains(upper, "O_WRONLY") || strings.Contains(upper, "O_RDWR") ||
		strings.Contains(upper, "O_CREAT") || strings.Contains(upper, "O_TRUNC") ||
		strings.Contains(upper, "O_APPEND") {
		return AccessWrite
	}
	return AccessRead
}

// extractQuotedStrings parses all C-escaped double-quoted strings from an arguments string.
func extractQuotedStrings(args string) []string {
	var result []string
	inQuote := false
	escaped := false
	var buf strings.Builder

	for i := 0; i < len(args); i++ {
		c := args[i]
		if inQuote {
			if escaped {
				switch c {
				case 'n':
					buf.WriteByte('\n')
				case 'r':
					buf.WriteByte('\r')
				case 't':
					buf.WriteByte('\t')
				case '\\':
					buf.WriteByte('\\')
				case '"':
					buf.WriteByte('"')
				default:
					buf.WriteByte('\\')
					buf.WriteByte(c)
				}
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inQuote = false
				result = append(result, buf.String())
				buf.Reset()
			} else {
				buf.WriteByte(c)
			}
		} else if c == '"' {
			inQuote = true
			escaped = false
			buf.Reset()
		}
	}

	return result
}

// extractSockAddr extracts socket family and destination address/path from connect/bind args.
func extractSockAddr(args string) string {
	sockPayload := args
	if idx := strings.Index(args, "{sa_family="); idx != -1 {
		sockPayload = args[idx:]
	} else if idx := strings.Index(args, "sa_family="); idx != -1 {
		sockPayload = args[idx:]
	}

	if strings.Contains(sockPayload, "AF_INET6") {
		return "AF_INET6:" + sockPayload
	}
	if strings.Contains(sockPayload, "AF_INET") {
		return "AF_INET:" + sockPayload
	}
	if strings.Contains(sockPayload, "AF_UNIX") {
		paths := extractQuotedStrings(sockPayload)
		if len(paths) > 0 {
			return "AF_UNIX:" + paths[0]
		}
		if aIdx := strings.Index(sockPayload, "sun_path=@"); aIdx != -1 {
			end := strings.IndexAny(sockPayload[aIdx:], ",})")
			if end != -1 {
				return "AF_UNIX:" + strings.TrimSpace(sockPayload[aIdx+9:aIdx+end])
			}
			return "AF_UNIX:" + strings.TrimSpace(sockPayload[aIdx+9:])
		}
		return "AF_UNIX:" + sockPayload
	}
	return args
}
