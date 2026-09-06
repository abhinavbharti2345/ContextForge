# Interceptor Hook Rules

When you are acting as an AI coding agent in this workspace, you must adhere to the following rule to prevent overriding historical architecture and decisions:

## Pre-Tool Use Requirement
Before using any file modification tool (e.g., `write_to_file`, `replace_in_file`, `edit_file`, or running a shell command to overwrite code), you MUST run the impact query command:

```bash
entire-agent-roo impact <target_file_path>
```

*(Note: If `entire-agent-roo` CLI is not mapped, you must find and execute the `QueryImpact` API or CLI entrypoint for the target file before making any edits.)*

### Handling Warnings
If the impact query returns a **🚨 SYSTEM WARNING**, you must:
1. **PAUSE** execution immediately.
2. Tell the user about the historical decisions and blast radius consequences found.
3. Ask for explicit permission to proceed with the edit, or propose an alternative approach that respects the constraints.

If it returns "No historical constraints.", you may proceed with the modification normally.
