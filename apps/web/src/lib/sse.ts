import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";

import type { SSEEvent, RequestGraph, WorkflowNodeData } from "./types";
import { authStore } from "./auth";

const RECONNECT_DELAY = 2000;
const POLL_INTERVAL = 3000;

function parseSSELine(line: string): { event?: string; data?: string } | null {
  if (line.startsWith("event: ")) {
    return { event: line.slice(7).trim() };
  }
  if (line.startsWith("data: ")) {
    return { data: line.slice(6).trim() };
  }
  return null;
}

/**
 * useRequestStream opens an EventSource for a request's SSE stream and
 * patches the TanStack Query cache for ["request", requestId] as events
 * arrive. It reconnects on error and falls back to polling if the EventSource
 * cannot connect.
 */
export function useRequestStream(requestId: string | null, enabled = true) {
  const queryClient = useQueryClient();
  const esRef = useRef<EventSource | null>(null);
  const pollTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    if (!requestId || !enabled) return;

    const token = authStore.get();
    if (!token) return;

    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    function patchCache(event: SSEEvent) {
      queryClient.setQueryData<RequestGraph>(["request", requestId], (old) => {
        if (!old) return old;

        switch (event.type) {
          case "node_status": {
            const nodes = old.nodes.map((n: WorkflowNodeData) =>
              n.id === event.node_id
                ? {
                    ...n,
                    status: (event.status ?? n.status) as WorkflowNodeData["status"],
                    progress_percent: event.progress_percent ?? n.progress_percent,
                    status_text: event.status_text ?? n.status_text,
                  }
                : n,
            );
            return { ...old, nodes };
          }
          case "request_status": {
            const status = event.status ?? old.request.status;
            return {
              ...old,
              request: {
                ...old.request,
                status: status as RequestGraph["request"]["status"],
                progress: event.progress_percent ?? old.request.progress,
              },
            };
          }
          // task and audit events are not yet consumed by the canvas
          // but accepted so future work (F5/F6) can reuse this hook.
          default:
            return old;
        }
      });
    }

    function connect() {
      const es = new EventSource(`/api/requests/${requestId}/events?token=${token}`);
      esRef.current = es;

      // Stop polling once the EventSource connects.
      if (pollTimer.current) {
        clearInterval(pollTimer.current);
        pollTimer.current = null;
      }

      es.addEventListener("node_status", (msg) => {
        try {
          patchCache(JSON.parse(msg.data) as SSEEvent);
        } catch { /* ignore malformed event */ }
      });

      es.addEventListener("request_status", (msg) => {
        try {
          patchCache(JSON.parse(msg.data) as SSEEvent);
        } catch { /* ignore malformed event */ }
      });

      es.addEventListener("error", () => {
        es.close();
        esRef.current = null;
        // Fall back to polling on error, then retry EventSource.
        startPolling();
        reconnectTimer = setTimeout(() => {
          connect();
        }, RECONNECT_DELAY);
      });
    }

    function startPolling() {
      if (pollTimer.current) return;
      pollTimer.current = setInterval(() => {
        // Invalidate the query so TanStack Query refetches.
        queryClient.invalidateQueries({ queryKey: ["request", requestId] });
      }, POLL_INTERVAL);
    }

    connect();

    return () => {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
      }
      if (pollTimer.current) {
        clearInterval(pollTimer.current);
        pollTimer.current = null;
      }
    };
  }, [requestId, enabled, queryClient]);
}
