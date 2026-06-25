import { useState } from "react";
import { motion } from "framer-motion";
import { AlertTriangle, Building2, Loader2, Pencil, Trash2 } from "lucide-react";

interface EditProjectModalProps {
  isOpen: boolean;
  project: { id: string; name: string; description: string };
  onClose: () => void;
  onConfirm: (name: string, description: string) => void;
  isPending: boolean;
}

export function EditProjectModal({
  isOpen,
  project,
  onClose,
  onConfirm,
  isPending,
}: EditProjectModalProps) {
  const [name, setName] = useState(project.name);
  const [description, setDescription] = useState(project.description);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="w-full max-w-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden"
      >
        <div className="p-5">
          <div className="flex items-center gap-2 text-[var(--color-brand)] mb-1">
            <Pencil size={16} />
            <h3 className="text-sm font-semibold">Edit Project</h3>
          </div>
          <p className="text-[12px] text-[var(--color-fg-muted)] mb-4 leading-relaxed">
            Update the project name and description.
          </p>

          <div className="space-y-3">
            <div>
              <label className="block text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)] mb-1 ml-1">
                Name
              </label>
              <input
                autoFocus
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-3 py-2 text-[13px] text-[var(--color-fg)] focus:outline-none focus:border-[var(--color-brand)]"
              />
            </div>
            <div>
              <label className="block text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)] mb-1 ml-1">
                Description
              </label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={2}
                className="w-full bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-md px-3 py-2 text-[12px] text-[var(--color-fg)] focus:outline-none focus:border-[var(--color-brand)] resize-none"
                placeholder="Optional description"
              />
            </div>
          </div>
        </div>

        <div className="bg-[var(--color-surface-2)] px-5 py-3 flex justify-end gap-2 border-t border-[var(--color-border)]">
          <button
            onClick={onClose}
            className="px-3 py-1.5 text-xs font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            Cancel
          </button>
          <button
            onClick={() => onConfirm(name.trim(), description.trim())}
            disabled={!name.trim() || isPending}
            className="px-4 py-1.5 text-xs font-medium bg-[var(--color-brand)] text-white rounded-md hover:brightness-110 disabled:opacity-40"
          >
            {isPending ? <Loader2 size={13} className="animate-spin" /> : "Save Changes"}
          </button>
        </div>
      </motion.div>
    </div>
  );
}

interface DeleteProjectModalProps {
  isOpen: boolean;
  project: { id: string; name: string };
  chatCount: number;
  onClose: () => void;
  onConfirm: () => void;
  isPending: boolean;
}

export function DeleteProjectModal({
  isOpen,
  project,
  chatCount,
  onClose,
  onConfirm,
  isPending,
}: DeleteProjectModalProps) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="w-full max-w-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden"
      >
        <div className="p-5">
          <div className="flex items-center gap-2 text-rose-500 mb-1">
            <AlertTriangle size={16} />
            <h3 className="text-sm font-semibold">Delete Project</h3>
          </div>
          <p className="text-[12px] text-[var(--color-fg-muted)] mb-4 leading-relaxed">
            This will permanently delete <strong>"{project.name}"</strong> and all its chats. This action cannot be undone.
          </p>
          {chatCount > 0 && (
            <div className="flex items-center gap-2 bg-rose-500/5 border border-rose-500/20 rounded-md px-3 py-2">
              <Building2 size={14} className="text-rose-500 shrink-0" />
              <span className="text-[12px] text-rose-600 dark:text-rose-300">
                This project contains {chatCount} {chatCount === 1 ? "chat" : "chats"} that will also be deleted.
              </span>
            </div>
          )}
        </div>

        <div className="bg-[var(--color-surface-2)] px-5 py-3 flex justify-end gap-2 border-t border-[var(--color-border)]">
          <button
            onClick={onClose}
            className="px-3 py-1.5 text-xs font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={isPending}
            className="px-4 py-1.5 text-xs font-medium bg-rose-500 text-white rounded-md hover:bg-rose-600 disabled:opacity-40"
          >
            {isPending ? <Loader2 size={13} className="animate-spin" /> : "Delete Project"}
          </button>
        </div>
      </motion.div>
    </div>
  );
}

interface DeleteChatModalProps {
  isOpen: boolean;
  chat: { id: string; title: string };
  onClose: () => void;
  onConfirm: () => void;
  isPending: boolean;
}

export function DeleteChatModal({
  isOpen,
  chat,
  onClose,
  onConfirm,
  isPending,
}: DeleteChatModalProps) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm">
      <motion.div
        initial={{ opacity: 0, scale: 0.95 }}
        animate={{ opacity: 1, scale: 1 }}
        className="w-full max-w-sm bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden"
      >
        <div className="p-5">
          <div className="flex items-center gap-2 text-rose-500 mb-1">
            <AlertTriangle size={16} />
            <h3 className="text-sm font-semibold">Delete Chat</h3>
          </div>
          <p className="text-[12px] text-[var(--color-fg-muted)] mb-4 leading-relaxed">
            This will permanently delete <strong>"{chat.title}"</strong> and all its messages. This action cannot be undone.
          </p>
        </div>

        <div className="bg-[var(--color-surface-2)] px-5 py-3 flex justify-end gap-2 border-t border-[var(--color-border)]">
          <button
            onClick={onClose}
            className="px-3 py-1.5 text-xs font-medium text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={isPending}
            className="px-4 py-1.5 text-xs font-medium bg-rose-500 text-white rounded-md hover:bg-rose-600 disabled:opacity-40"
          >
            {isPending ? <Loader2 size={13} className="animate-spin" /> : "Delete Chat"}
          </button>
        </div>
      </motion.div>
    </div>
  );
}