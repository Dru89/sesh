import { useState } from "react";
import { ActionPanel, Action, Icon, List, Color } from "@raycast/api";
import { useCachedPromise } from "@raycast/utils";
import { loadSessions, relativeTime } from "./sesh";
import { openInTerminal } from "./terminal";
import { SeshSession } from "./types";

const AGENT_COLORS: Record<string, Color> = {
  opencode: Color.Blue,
  claude: Color.Purple,
};

function agentColor(agent: string): Color {
  return AGENT_COLORS[agent] ?? Color.Yellow;
}

function abbreviateHome(path: string): string {
  const home = process.env.HOME;
  if (home && path.startsWith(home)) {
    return "~" + path.slice(home.length);
  }
  return path;
}

function displayTitle(session: SeshSession): string {
  return session.summary || session.title || session.slug || session.id;
}

function sessionDetailMarkdown(session: SeshSession): string {
  const lines: string[] = [];

  lines.push(`## ${displayTitle(session)}`);
  lines.push("");
  lines.push(`| Field | Value |`);
  lines.push(`|---|---|`);
  lines.push(`| **Agent** | ${session.agent} |`);
  lines.push(`| **Session ID** | \`${session.id}\` |`);
  if (session.slug) {
    lines.push(`| **Slug** | ${session.slug} |`);
  }
  if (session.directory) {
    lines.push(`| **Directory** | \`${abbreviateHome(session.directory)}\` |`);
  }
  lines.push(`| **Created** | ${new Date(session.created).toLocaleString()} |`);
  lines.push(`| **Last Used** | ${new Date(session.last_used).toLocaleString()} (${relativeTime(session.last_used)}) |`);
  lines.push("");
  lines.push("**Resume command:**");
  lines.push("```");
  lines.push(session.resume_command);
  lines.push("```");

  if (session.title && session.summary && session.title !== session.summary) {
    lines.push("");
    lines.push(`**Original title:** ${session.title}`);
  }

  return lines.join("\n");
}

export default function SearchSessions() {
  const [showDetail, setShowDetail] = useState(false);
  const { data: sessions, isLoading } = useCachedPromise(
    async () => loadSessions(),
    [],
    { keepPreviousData: true }
  );

  return (
    <List
      isLoading={isLoading}
      isShowingDetail={showDetail}
      searchBarPlaceholder="Search sessions..."
    >
      {sessions?.map((session) => {
        const props: Partial<List.Item.Props> = showDetail
          ? {
              detail: (
                <List.Item.Detail markdown={sessionDetailMarkdown(session)} />
              ),
            }
          : {
              subtitle: session.directory
                ? abbreviateHome(session.directory)
                : undefined,
              accessories: [
                {
                  tag: {
                    value: session.agent,
                    color: agentColor(session.agent),
                  },
                },
                { text: relativeTime(session.last_used) },
              ],
            };

        return (
          <List.Item
            key={`${session.agent}-${session.id}`}
            id={`${session.agent}-${session.id}`}
            title={displayTitle(session)}
            icon={{
              source: Icon.Terminal,
              tintColor: agentColor(session.agent),
            }}
            keywords={[
              session.agent,
              session.slug ?? "",
              session.directory ?? "",
              session.title,
              session.summary ?? "",
            ].filter(Boolean)}
            {...props}
            actions={
              <ActionPanel>
                <ActionPanel.Section title="Resume">
                  <Action
                    title="Resume Session"
                    icon={Icon.Terminal}
                    onAction={() => openInTerminal(session.resume_command)}
                  />
                  <Action
                    title="Toggle Detail"
                    icon={Icon.Sidebar}
                    shortcut={{ modifiers: ["cmd"], key: "d" }}
                    onAction={() => setShowDetail(!showDetail)}
                  />
                </ActionPanel.Section>
                <ActionPanel.Section title="Copy">
                  <Action.CopyToClipboard
                    title="Copy Resume Command"
                    content={session.resume_command}
                    shortcut={{ modifiers: ["cmd", "shift"], key: "c" }}
                  />
                  <Action.CopyToClipboard
                    title="Copy Session ID"
                    content={session.id}
                    shortcut={{ modifiers: ["cmd"], key: "i" }}
                  />
                  {session.directory && (
                    <Action.CopyToClipboard
                      title="Copy Directory"
                      content={session.directory}
                    />
                  )}
                </ActionPanel.Section>
                {session.directory && (
                  <ActionPanel.Section title="Open">
                    <Action.ShowInFinder
                      title="Open Directory in Finder"
                      path={session.directory}
                      shortcut={{ modifiers: ["cmd"], key: "o" }}
                    />
                    <Action.Open
                      title="Open Directory in VS Code"
                      target={session.directory}
                      application="Visual Studio Code"
                      shortcut={{ modifiers: ["cmd", "shift"], key: "o" }}
                    />
                  </ActionPanel.Section>
                )}
              </ActionPanel>
            }
          />
        );
      })}
    </List>
  );
}
