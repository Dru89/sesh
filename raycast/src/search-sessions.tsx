import { useState } from "react";
import { Action, ActionPanel, Icon, List, showToast, Toast } from "@raycast/api";
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
  const [filteredEmpty, setFilteredEmpty] = useState(false);

  const { data: sessions, isLoading } = useCachedPromise(
    async () => loadSessions(),
    [],
    { keepPreviousData: true }
  );

  const displaySessions = aiResults ?? sessions ?? [];
  const isAiMode = aiResults !== null;

  async function handleAiSearch() {
    if (!searchText.trim()) return;
    setAiLoading(true);

    const results = await aiSearchSessions(searchText);
    setAiResults(results);
    setAiLoading(false);
  }

  function handleSearchChange(text: string) {
    setSearchText(text);
    setFilteredEmpty(false);
    if (aiResults !== null) {
      setAiResults(null);
    }
  }

  // Raycast calls onSelectionChange with null when all items are filtered out.
  function handleSelectionChange(id: string | null) {
    if (id === null && searchText.length >= 3 && !isLoading && displaySessions.length > 0) {
      setFilteredEmpty(true);
    } else {
      setFilteredEmpty(false);
    }
  }

  return (
    <List
      isLoading={isLoading || aiLoading}
      isShowingDetail={showDetail}
      searchBarPlaceholder="Search sessions..."
      onSearchTextChange={handleSearchChange}
      onSelectionChange={handleSelectionChange}
      filtering={!isAiMode}
      navigationTitle={isAiMode ? "Search Sessions (AI)" : "Search Sessions"}
      actions={
        filteredEmpty ? (
          <ActionPanel>
            <Action
              title="Search with AI"
              icon={Icon.Stars}
              onAction={handleAiSearch}
            />
          </ActionPanel>
        ) : undefined
      }
    >
      {displaySessions.map((session) => (
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
      ))}
    </List>
  );
}
