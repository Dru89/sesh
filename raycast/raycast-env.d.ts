/// <reference types="@raycast/api">

/* 🚧 🚧 🚧
 * This file is auto-generated from the extension's manifest.
 * Do not modify manually. Instead, update the `package.json` file.
 * 🚧 🚧 🚧 */

/* eslint-disable @typescript-eslint/ban-types */

type ExtensionPreferences = {
  /** sesh Binary Path - Path to the sesh binary. Leave empty to use the default from PATH. */
  "seshPath": string,
  /** Terminal Application - Which terminal to use when resuming a session */
  "terminal": "terminal" | "iterm" | "ghostty" | "warp" | "custom",
  /** Custom Terminal Command - AppleScript or shell command to open a terminal and run a command. Use {cmd} as placeholder for the resume command. Only used when Terminal is set to Custom. */
  "customTerminalCommand": string
}

/** Preferences accessible in all the extension's commands */
declare type Preferences = ExtensionPreferences

declare namespace Preferences {
  /** Preferences accessible in the `search-sessions` command */
  export type SearchSessions = ExtensionPreferences & {}
}

declare namespace Arguments {
  /** Arguments passed to the `search-sessions` command */
  export type SearchSessions = {}
}

