package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"bws/internal/config"
	"bws/internal/sandbox"
)

// HandleConfigWhere prints the filepaths for global and local configs and skeletons.
func HandleConfigWhere() {
	cwd, _ := os.Getwd()
	fmt.Printf("Global:          %s\n", config.GlobalPath())
	fmt.Printf("Global Skeleton: %s\n", sandbox.GlobalSkeletonDir())
	fmt.Printf("Local:           %s\n", config.FindLocalPath(cwd))
	fmt.Printf("Local Skeleton:  %s\n", sandbox.LocalSkeletonDir(cwd))
}

// HandleConfigShow prints the raw content of the target configuration file.
func HandleConfigShow(global, local bool) {
	if !global && !local {
		local = true
	}
	path := configFilePath(global)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			label := "Global"
			if !global {
				label = "Local"
			}
			fmt.Printf("%s config not found at %s.\n", label, path)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(string(data))
}

// HandleConfigReset resets the target configuration file to clean defaults.
func HandleConfigReset(global, local bool) {
	if !global && !local {
		local = true
	}
	path := configFilePath(global)
	label := "global"
	if !global {
		label = "local"
	}

	if _, err := os.Stat(path); err == nil {
		backup := path + ".bak"
		if err := os.Rename(path, backup); err != nil {
			fmt.Fprintf(os.Stderr, "Error backing up config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Backed up old %s config to %s\n", label, backup)
	}

	if global {
		config.CreateDefault(path)
		examplePath := filepath.Join(filepath.Dir(path), "example-config.jsonc")
		config.CreateExampleConfig(examplePath)
		sandbox.EnsureGlobalSkeleton()
	} else {
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(config.ExampleConfigContent), 0644)
	}

	fmt.Printf("Created clean %s config: %s\n", label, path)
}

// HandleConfigEdit opens the target config file in $EDITOR.
func HandleConfigEdit(global, local bool) {
	if !global && !local {
		local = true
	}
	path := configFilePath(global)
	label := "global"
	if !global {
		label = "local"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "%s config not found at %s. Run 'bws config reset' first.\n", label, path)
		os.Exit(1)
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
		os.Exit(1)
	}
}

// HandleConfigSet sets a configuration key value in target config.
func HandleConfigSet(key, value string, global, local bool) {
	if !global && !local {
		local = true
	}
	path := configFilePath(global)
	label := "global"
	if !global {
		label = "local"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if global {
			config.CreateDefault(path)
		} else {
			_ = os.MkdirAll(filepath.Dir(path), 0755)
			_ = os.WriteFile(path, []byte("{\n}\n"), 0644)
		}
	}

	if err := config.SetConfigKV(path, key, value); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting config key %q: %v\n", key, err)
		os.Exit(1)
	}

	fmt.Printf("Set %s in %s configuration (%s) = %s\n", key, label, path, value)
}

// HandleConfigGet reads a configuration key value from target config.
func HandleConfigGet(key string, global, local bool) {
	if !global && !local {
		local = true
	}
	path := configFilePath(global)
	label := "global"
	if !global {
		label = "local"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "%s config not found at %s\n", label, path)
		os.Exit(1)
	}

	val, err := config.GetConfigKV(path, key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", key, err)
		os.Exit(1)
	}

	fmt.Println(val)
}

// HandleConfigUnset removes a configuration key from target config.
func HandleConfigUnset(key string, global, local bool) {
	if !global && !local {
		local = true
	}
	path := configFilePath(global)
	label := "global"
	if !global {
		label = "local"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "%s config not found at %s\n", label, path)
		os.Exit(1)
	}

	if err := config.UnsetConfigKV(path, key); err != nil {
		fmt.Fprintf(os.Stderr, "Error unsetting %s: %v\n", key, err)
		os.Exit(1)
	}

	fmt.Printf("Unset %s from %s configuration (%s)\n", key, label, path)
}

// HandleConfigPush copies global configuration & theme files to a remote host via SCP.
func HandleConfigPush(destination string) {
	HandleSCP([]string{destination})
}

func HandleConfigShowGlobal() {
	HandleConfigShow(true, false)
}

func HandleConfigShowLocal() {
	HandleConfigShow(false, true)
}

func HandleConfigInitGlobal() {
	HandleConfigReset(true, false)
}

func HandleConfigInitLocal() {
	HandleConfigReset(false, true)
}

func HandleConfigEditGlobal() {
	HandleConfigEdit(true, false)
}

func HandleConfigEditLocal() {
	HandleConfigEdit(false, true)
}
