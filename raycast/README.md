# sesh Raycast Extension

Browse and resume coding agent sessions from Raycast. Searches across OpenCode, Claude Code, and any other agent configured in sesh.

## Requirements

- [sesh](https://github.com/dru89/sesh) must be installed and on your PATH
- At least one coding agent (OpenCode, Claude Code, etc.) with existing sessions

## Install (dev mode)

1. Clone the sesh repo (if you haven't already):
   ```bash
   git clone https://github.com/dru89/sesh.git
   ```

2. Install dependencies:
   ```bash
   cd sesh/raycast
   npm install
   ```

3. Open Raycast, go to Extensions, and import the extension from the `raycast/` directory.

Or from Raycast's developer tools: run `npm run dev` in the `raycast/` directory.

## Configuration

Open the extension preferences in Raycast to configure:

- **sesh Binary Path** — path to the sesh binary. Leave empty if sesh is on your PATH. If sesh is installed via `go install`, you may need to set this to `~/go/bin/sesh`.
- **Terminal Application** — which terminal opens when you resume a session. Options: Terminal.app, iTerm2, Ghostty, Warp, or Custom.
- **Custom Terminal Command** — only used when Terminal is set to "Custom". Use `{cmd}` as a placeholder for the resume command. Example: `osascript -e 'tell application "Alacritty" to do script "{cmd}"'`

## Usage

1. Open Raycast and search for "Search Sessions" (or set a hotkey)
2. Type to filter sessions by title, agent, directory, or summary
3. Press Enter to resume the selected session in your terminal
4. Use the action panel (Cmd+K) for additional actions:
   - Copy resume command
   - Copy session ID
   - Open directory in Finder
   - Open directory in VS Code

## How it works

The extension runs `sesh --json` to get the full session list, then presents it using Raycast's built-in List view with fuzzy search. When you select a session, it uses AppleScript (for Terminal.app and iTerm2), CLI flags (for Ghostty), or deep links (for Warp) to open a new terminal window and run the resume command.
