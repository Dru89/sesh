import { getPreferenceValues, showToast, Toast } from "@raycast/api";
import { execSync } from "child_process";

interface TerminalPrefs {
  terminal: "terminal" | "iterm" | "ghostty" | "warp" | "custom";
  customTerminalCommand?: string;
}

/**
 * Opens the specified terminal and runs a command in it.
 * The command is the resume_command from sesh --json output.
 */
export function openInTerminal(command: string): void {
  const prefs = getPreferenceValues<TerminalPrefs>();

  try {
    switch (prefs.terminal) {
      case "terminal":
        openInTerminalApp(command);
        break;
      case "iterm":
        openInITerm(command);
        break;
      case "ghostty":
        openInGhostty(command);
        break;
      case "warp":
        openInWarp(command);
        break;
      case "custom":
        openInCustom(command, prefs.customTerminalCommand || "");
        break;
      default:
        openInTerminalApp(command);
    }
  } catch (err) {
    showToast({
      style: Toast.Style.Failure,
      title: "Failed to open terminal",
      message: err instanceof Error ? err.message : String(err),
    });
  }
}

function openInTerminalApp(command: string): void {
  const escaped = command.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  execSync(
    `osascript -e 'tell application "Terminal"
      activate
      do script "${escaped}"
    end tell'`,
    { timeout: 5000 }
  );
}

function openInITerm(command: string): void {
  const escaped = command.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  execSync(
    `osascript -e 'tell application "iTerm"
      activate
      create window with default profile command "${escaped}"
    end tell'`,
    { timeout: 5000 }
  );
}

function openInGhostty(command: string): void {
  // Ghostty supports launching with a command via its CLI.
  const escaped = command.replace(/'/g, "'\\''");
  execSync(`open -a Ghostty --args -e '/bin/bash' '-c' '${escaped}'`, { timeout: 5000 });
}

function openInWarp(command: string): void {
  // Warp supports deep links for running commands.
  const encoded = encodeURIComponent(command);
  execSync(`open "warp://action/launch?command=${encoded}"`, { timeout: 5000 });
}

function openInCustom(command: string, template: string): void {
  if (!template) {
    showToast({
      style: Toast.Style.Failure,
      title: "No custom terminal command configured",
      message: "Set a custom terminal command in the extension preferences.",
    });
    return;
  }
  const fullCommand = template.replace("{cmd}", command);
  execSync(fullCommand, { timeout: 5000, shell: "/bin/bash" });
}
