import { useState } from "react";
import { Action, ActionPanel, Icon, List } from "@raycast/api";
import { useCachedPromise } from "@raycast/utils";
import { loadSessions, aiSearchSessions } from "./sesh";
import {
  agentColor,
  displayTitle,
  SessionActions,
  sessionListItemProps,
} from "./components";
import { SeshSession } from "./types";

export default function SearchSessions() {
  const [showDetail, setShowDetail] = useState(false);
  const [searchText, setSearchText] = useState("");
  const [aiResults, setAiResults] = useState<SeshSession[] | null>(null);
  const [aiLoading, setAiLoading] = useState(false);

  const { data: sessions, isLoading } = useCachedPromise(
    async () => loadSessions(),
    [],
    { keepPreviousData: true }
  );

  const displaySessions = aiResults ?? sessions ?? [];
  const isAiMode = aiResults !== null;

  function handleAiSearch() {
    if (!searchText.trim()) return;
    setAiLoading(true);
    // Run async to not block the UI thread.
    setTimeout(() => {
      const results = aiSearchSessions(searchText);
      setAiResults(results);
      setAiLoading(false);
    }, 0);
  }

  function handleSearchChange(text: string) {
    setSearchText(text);
    // Clear AI results when the user changes the search text.
    if (aiResults !== null) {
      setAiResults(null);
    }
  }

  return (
    <List
      isLoading={isLoading || aiLoading}
      isShowingDetail={showDetail}
      searchBarPlaceholder="Search sessions..."
      onSearchTextChange={handleSearchChange}
      filtering={!isAiMode}
      navigationTitle={isAiMode ? "Search Sessions (AI)" : "Search Sessions"}
    >
      {displaySessions.length === 0 && !isLoading && !aiLoading && searchText.length >= 3 ? (
        <List.EmptyView
          icon={Icon.MagnifyingGlass}
          title="No matching sessions"
          description="Press Enter to search with AI"
          actions={
            <ActionPanel>
              <Action
                title="Search with AI"
                icon={Icon.Stars}
                onAction={handleAiSearch}
              />
            </ActionPanel>
          }
        />
      ) : (
        displaySessions.map((session) => (
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
            {...sessionListItemProps(session, showDetail)}
            actions={
              <SessionActions
                session={session}
                showDetail={showDetail}
                onToggleDetail={() => setShowDetail(!showDetail)}
                extraActions={
                  <ActionPanel.Section>
                    <Action
                      title="Search with AI"
                      icon={Icon.Stars}
                      shortcut={{ modifiers: ["cmd", "shift"], key: "a" }}
                      onAction={handleAiSearch}
                    />
                  </ActionPanel.Section>
                }
              />
            }
          />
        ))
      )}
    </List>
  );
}
