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

export default function SearchSessions() {
  const { data: sessions, isLoading } = useCachedPromise(
    async () => loadSessions(),
    [],
    { keepPreviousData: true }
  );

  return (
    <List
      isLoading={isLoading}
      searchBarPlaceholder="Search sessions..."
    >
      {sessions?.map((session) => (
        <List.Item
          key={`${session.agent}-${session.id}`}
          id={`${session.agent}-${session.id}`}
          title={displayTitle(session)}
          subtitle={session.directory ? abbreviateHome(session.directory) : undefined}
          icon={{ source: Icon.Terminal, tintColor: agentColor(session.agent) }}
          keywords={[
            session.agent,
            session.slug ?? "",
            session.directory ?? "",
            session.title,
            session.summary ?? "",
          ].filter(Boolean)}
          accessories={[
            { tag: { value: session.agent, color: agentColor(session.agent) } },
            { text: relativeTime(session.last_used) },
          ]}
          actions={
            <ActionPanel>
              <ActionPanel.Section title="Resume">
                <Action
                  title="Resume Session"
                  icon={Icon.Terminal}
                  onAction={() => openInTerminal(session.resume_command)}
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
      ))}
    </List>
  );
}
