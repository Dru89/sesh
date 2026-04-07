import { useState, useEffect, useRef } from "react";
import { Icon, List } from "@raycast/api";
import { aiSearchSessions } from "./sesh";
import {
  agentColor,
  displayTitle,
  SessionActions,
  sessionListItemProps,
} from "./components";
import { SeshSession } from "./types";

export default function AiSearchSessions() {
  const [showDetail, setShowDetail] = useState(false);
  const [searchText, setSearchText] = useState("");
  const [results, setResults] = useState<SeshSession[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [hasSearched, setHasSearched] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const abortRef = useRef(false);

  useEffect(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
    }
    // Abort any in-flight search.
    abortRef.current = true;

    if (searchText.trim().length < 3) {
      setResults([]);
      setHasSearched(false);
      setIsLoading(false);
      return;
    }

    // Debounce: wait 600ms after the user stops typing.
    timerRef.current = setTimeout(async () => {
      abortRef.current = false;
      setIsLoading(true);
      setHasSearched(true);

      const sessions = await aiSearchSessions(searchText);

      // Don't update if the search was superseded by a newer one.
      if (!abortRef.current) {
        setResults(sessions);
        setIsLoading(false);
      }
    }, 600);

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
      }
    };
  }, [searchText]);

  return (
    <List
      isLoading={isLoading}
      isShowingDetail={showDetail}
      filtering={false}
      onSearchTextChange={setSearchText}
      searchBarPlaceholder="Ask about your sessions..."
      navigationTitle="AI Search Sessions"
    >
      {results.length === 0 && !isLoading ? (
        <List.EmptyView
          icon={Icon.Stars}
          title={
            hasSearched
              ? "No relevant sessions found"
              : "Type a question to search with AI"
          }
          description={
            hasSearched
              ? "Try a different query"
              : 'e.g. "auth token refresh work last week"'
          }
        />
      ) : (
        results.map((session) => (
          <List.Item
            key={`${session.agent}-${session.id}`}
            id={`${session.agent}-${session.id}`}
            title={displayTitle(session)}
            icon={{
              source: Icon.Terminal,
              tintColor: agentColor(session.agent),
            }}
            {...sessionListItemProps(session, showDetail)}
            actions={
              <SessionActions
                session={session}
                showDetail={showDetail}
                onToggleDetail={() => setShowDetail(!showDetail)}
              />
            }
          />
        ))
      )}
    </List>
  );
}
