# Claude 3.7 Sonnet Deep Documentation & DevEx Review Prompt

Use this prompt when running a deep, detailed, and editorial documentation review with **Claude 3.7 Sonnet (Extended Thinking)**:

---

```markdown
You are a Principal Technical Writer, Developer Experience (DevEx) Architect, and Senior Systems Documentation Engineer. 

Perform a deep, exhaustive, line-by-line editorial and usability review of all documentation files in the `bws` (Bubblewrap Sandbox) repository:
1. `README.md` (Main entry point & Quick Start)
2. `docs/faq.md` (Design rationale, architectural trade-offs, common questions)
3. `docs/configuration.md` (JSONC schema, layering rules, dynamic token expansion)
4. `docs/security.md` (Threat model, path masking mechanics, hardening profiles)
5. `docs/commands.md` (CLI command, subcommand, and flag reference)
6. `docs/architecture.md` (Linux namespace internals, invocation lifecycle)
7. `profiles/README.md` (35+ capability profile catalog & verification specs)

---

### Audit Dimensions

#### 1. Time-to-First-Value & Developer Onboarding Flow
* Can a developer with zero prior knowledge of Bubblewrap understand the value proposition within 30 seconds and run their first isolated sandbox within 60 seconds?
* Are prerequisite steps (distro package managers, `$PATH` export for `~/bin`, `gh` authentication) unambiguous and friction-free?
* Is the Quick Start sequenced in the exact logical order a developer needs?

#### 2. Cognitive Friction, Tone & Prose Quality
* Eliminate any remaining marketing superlatives, fluff, run-on sentences, and awkward passive phrasing.
* Ensure clear, direct, idiomatic systems-engineering prose.
* Verify that technical terminology (e.g. user namespaces, tmpfs overlays, ephemeral staging, DAG resolution, Deploy Keys) is explained with maximum clarity and zero confusion.

#### 3. Information Architecture & Progressive Disclosure
* Does `README.md` maintain a concise executive summary, properly delegating deep dives to the `docs/` hierarchy?
* Are headings clean, sentence-cased, and semantic?
* Are cross-document relative links (`[link](docs/...)`) accurate, contextual, and intuitive?

#### 4. Code Snippets, Tables & Visual Diagram Usability
* Are all CLI shell commands copy-pasteable, realistic, and accompanied by explanations of their side-effects?
* Are Mermaid diagrams syntactically valid, readable on both light and dark themes, and accurately reflecting the system architecture?
* Are JSON / JSONC configuration snippets valid and commented informatively?

#### 5. Omissions, Edge Cases & Troubleshooting
* What questions would an experienced developer, security engineer, or AI agent builder have that are currently left unanswered?
* Are failure modes and recovery paths documented (e.g., missing dependencies, permissions, SSH remote formats)?

---

### Required Output Structure

1. **Executive DevEx Scorecard**: Rate (1-10) with concise justifications across:
   * *Clarity & Readability*
   * *First-Run Usability*
   * *Technical Precision*
   * *Information Architecture*
2. **Top High-Impact Recommendations**: The 3–5 most impactful improvements to elevate the documentation to tier-1 open-source standard.
3. **File-by-File Surgical Line Edits**: Line-by-line critique with precise before/after markdown diff blocks for any sections that need improvement.
4. **Concrete Rewrite Proposals**: Full replacement text for any weak sections or missing troubleshooting guides.
```
