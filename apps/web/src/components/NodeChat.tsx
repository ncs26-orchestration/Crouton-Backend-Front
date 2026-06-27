import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Bot, Send, RefreshCw } from "lucide-react";

import { api } from "../lib/api";
import { useToasts } from "../components/Toasts";
import { Avatar } from "./Avatar";

// The per-node conversation between a verifier and the department agent. The
// verifier can ask a question (the agent answers) or request changes (the agent
// revises its decision). Disabled when the viewer may not act on the node.
export function NodeChat({
  requestId,
  nodeId,
  canPost,
}: {
  requestId: string;
  nodeId: string;
  canPost: boolean;
}) {
  const qc = useQueryClient();
  const toasts = useToasts();
  const [text, setText] = useState("");

  const { data } = useQuery({
    queryKey: ["node-messages", requestId, nodeId],
    queryFn: () => api.listNodeMessages(requestId, nodeId),
  });
  const messages = data?.messages ?? [];

  const post = useMutation({
    mutationFn: (intent: "question" | "request_changes") =>
      api.postNodeMessage(requestId, nodeId, { body: text.trim(), intent }),
    onSuccess: (res) => {
      setText("");
      qc.setQueryData(["node-messages", requestId, nodeId], res);
      // A revise updates the node's decision, so refresh the graph + node detail.
      qc.invalidateQueries({ queryKey: ["request", requestId] });
      qc.invalidateQueries({ queryKey: ["node", requestId, nodeId] });
    },
    onError: (e: Error) => toasts.push({ kind: "error", title: e.message }),
  });

  return (
    <div className="px-4 py-3 border-t border-[var(--color-border)]">
      <h4 className="text-[10px] uppercase tracking-wide text-[var(--color-fg-muted)] mb-2">
        Discuss with the agent
      </h4>

      {messages.length > 0 && (
        <div className="flex flex-col gap-2.5 mb-3 max-h-56 overflow-y-auto nice-scroll">
          {messages.map((m) => (
            <div key={m.id} className="flex gap-2 text-xs">
              {m.role === "human" ? (
                <Avatar name={m.author_name || "You"} size={20} />
              ) : (
                <span className="size-5 shrink-0 rounded-full bg-[var(--color-accent-bg)] flex items-center justify-center">
                  <Bot size={11} className="text-[var(--color-brand)]" />
                </span>
              )}
              <div className="min-w-0">
                <p className="text-[10px] text-[var(--color-fg-muted)]">
                  {m.role === "human" ? m.author_name : m.author_name || "Agent"}
                </p>
                <p
                  className={`leading-snug ${m.role === "system" ? "italic text-[var(--color-fg-muted)]" : "text-[var(--color-fg)]"}`}
                >
                  {m.body}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}

      {canPost ? (
        <div className="flex flex-col gap-2">
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={2}
            placeholder="Ask the agent, or describe a change…"
            className="w-full rounded border border-[var(--color-border)] bg-[var(--color-bg)] px-2.5 py-1.5 text-xs text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] focus:outline-none focus:ring-1 focus:ring-[var(--color-brand)] resize-none"
          />
          <div className="flex gap-2">
            <button
              onClick={() => post.mutate("question")}
              disabled={!text.trim() || post.isPending}
              className="flex-1 flex items-center justify-center gap-1 rounded-md border border-[var(--color-border)] px-2 py-1.5 text-xs font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:border-[var(--color-border-strong)] disabled:opacity-40"
            >
              <Send size={12} /> Ask
            </button>
            <button
              onClick={() => post.mutate("request_changes")}
              disabled={!text.trim() || post.isPending}
              className="flex-1 flex items-center justify-center gap-1 rounded-md bg-[var(--color-brand)] px-2 py-1.5 text-xs font-medium text-white hover:bg-[var(--color-brand-hover)] disabled:opacity-40"
            >
              <RefreshCw size={12} /> Request changes
            </button>
          </div>
        </div>
      ) : (
        messages.length === 0 && (
          <p className="text-[11px] text-[var(--color-fg-subtle)]">
            Only this step's department or an executive can chat with the agent.
          </p>
        )
      )}
    </div>
  );
}
