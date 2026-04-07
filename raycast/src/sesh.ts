import { execSync } from "child_process";
import { getPreferenceValues, showToast, Toast } from "@raycast/api";
import { SeshSession } from "./types";

export function getSeshPath(): string {
  const prefs = getPreferenceValues<{ seshPath?: string }>();
  return prefs.seshPath || "sesh";
}

function seshEnv(): NodeJS.ProcessEnv {
  return {
    ...process.env,
    PATH: [
      process.env.PATH,
      "/usr/local/bin",
      "/opt/homebrew/bin",
      `${process.env.HOME}/.local/bin`,
      `${process.env.HOME}/go/bin`,
      `${process.env.HOME}/.opencode/bin`,
    ].join(":"),
  };
}

export function loadSessions(): SeshSession[] {
  const sesh = getSeshPath();
  try {
    const output = execSync(`${sesh} --json`, {
      timeout: 10000,
      encoding: "utf-8",
      shell: "/bin/bash",
      env: seshEnv(),
    });
    return JSON.parse(output);
  } catch (err) {
    showToast({
      style: Toast.Style.Failure,
      title: "Failed to load sessions",
      message: err instanceof Error ? err.message : String(err),
    });
    return [];
  }
}

export function aiSearchSessions(query: string): SeshSession[] {
  const sesh = getSeshPath();
  try {
    const output = execSync(`${sesh} --json --ai-search ${shellQuote(query)}`, {
      timeout: 30000,
      encoding: "utf-8",
      shell: "/bin/bash",
      env: seshEnv(),
    });
    return JSON.parse(output);
  } catch (err) {
    showToast({
      style: Toast.Style.Failure,
      title: "AI search failed",
      message: err instanceof Error ? err.message : String(err),
    });
    return [];
  }
}

export function relativeTime(isoDate: string): string {
  const d = Date.now() - new Date(isoDate).getTime();
  if (d < 60_000) return "just now";
  if (d < 3_600_000) return `${Math.floor(d / 60_000)}m ago`;
  if (d < 86_400_000) return `${Math.floor(d / 3_600_000)}h ago`;
  if (d < 30 * 86_400_000) return `${Math.floor(d / 86_400_000)}d ago`;
  return new Date(isoDate).toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function shellQuote(s: string): string {
  return "'" + s.replace(/'/g, "'\\''") + "'";
}
