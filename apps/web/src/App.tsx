import { useEffect, useState } from "react";
import { ReactFlowProvider } from "@xyflow/react";

import { HelpOverlay } from "./components/HelpOverlay";
import { OrgSettingsModal } from "./components/OrgSettingsModal";
import { ProjectTree } from "./components/ProjectTree";
import { ShellRail, type ShellSection } from "./components/ShellRail";
import { ToastProvider } from "./components/Toasts";
import { ThemeProvider } from "./lib/theme";
import { api } from "./lib/api";
import type { Chat } from "./lib/types";
import { ChatView } from "./views/ChatView";
import { ProjectsHomeView } from "./views/ProjectsHomeView";
import { SettingsView } from "./views/SettingsView";

// App shell — keeps the operator's three persistent surfaces
// always reachable from the left icon rail:
//
//   Home     — projects grid or the current chat, depending on
//              whether a chat is selected.
//   Settings — deploy-target connectors (per project) + theme.
//   Help     — the existing keyboard-shortcut overlay.
//
// Navigation state is just three things: which section, which
// project is scoped, which chat (if any) is open. All persisted in
// localStorage so refreshes land back in the same place.

const STORAGE_KEY = "aup.lastLocation";

interface Location {
  section: ShellSection;
  projectId: string | null;
  chatId: string | null;
}

function loadLocation(): Location {
  if (typeof window === "undefined") {
    return { section: "home", projectId: null, chatId: null };
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return { section: "home", projectId: null, chatId: null };
    const parsed = JSON.parse(raw);
    return {
      section: parsed.section === "settings" || parsed.section === "help" ? parsed.section : "home",
      projectId: typeof parsed.projectId === "string" ? parsed.projectId : null,
      chatId: typeof parsed.chatId === "string" ? parsed.chatId : null,
    };
  } catch {
    return { section: "home", projectId: null, chatId: null };
  }
}

function saveLocation(loc: Location) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(loc));
}

export function App() {
  return (
    <ThemeProvider>
      <ToastProvider>
        <ReactFlowProvider>
          <Shell />
        </ReactFlowProvider>
      </ToastProvider>
    </ThemeProvider>
  );
}

function Shell() {
  const [location, setLocation] = useState<Location>(loadLocation);
  const [helpOpen, setHelpOpen] = useState(false);
  const [orgModalProject, setOrgModalProject] = useState<{ id: string; name: string } | null>(null);

  useEffect(() => {
    saveLocation(location);
  }, [location]);

  const setSection = (section: ShellSection) => {
    // Opening Help is transient — don't persist it; it pops a modal
    // and the previous section stays underneath.
    if (section === "help") {
      setHelpOpen(true);
      return;
    }
    setLocation((prev) => ({ ...prev, section }));
  };

  const openProject = (projectId: string) =>
    setLocation({ section: "home", projectId, chatId: null });

  const openChat = (chat: Chat) =>
    setLocation({ section: "home", projectId: chat.project_id, chatId: chat.id });

  const backToProjects = () =>
    setLocation({ section: "home", projectId: null, chatId: null });

  const openOrganisationSettings = () => {
    if (location.projectId) {
      setOrgModalProject({ id: location.projectId, name: "" });
    }
  };

  return (
    <div className="h-screen w-screen overflow-hidden flex bg-[var(--color-bg)] text-[var(--color-fg)]">
      <ShellRail
        active={location.section}
        onSelect={setSection}
        onBrandClick={backToProjects}
      />

      {location.section === "home" && (
        <>
          <ProjectTree
            selectedProjectId={location.projectId}
            selectedChatId={location.chatId}
            onOpenProject={openProject}
            onOpenChat={openChat}
            onBackToProjects={backToProjects}
            onOrganisationSettings={location.projectId ? openOrganisationSettings : undefined}
          />
          {location.chatId ? (
            <ChatView chatId={location.chatId} />
          ) : (
            <ProjectsHomeView onOpenChat={openChat} onOpenProject={openProject} />
          )}
        </>
      )}

      {location.section === "settings" && (
        <SettingsView scopedProjectId={location.projectId} />
      )}

      <HelpOverlay open={helpOpen} onClose={() => setHelpOpen(false)} />

      {orgModalProject && (
        <OrgSettingsModalWrapper
          projectId={orgModalProject.id}
          onClose={() => setOrgModalProject(null)}
        />
      )}
    </div>
  );
}

function OrgSettingsModalWrapper({ projectId, onClose }: { projectId: string; onClose: () => void }) {
  const [orgModalProject, setOrgModalProject] = useState<{ id: string; name: string } | null>(null);

  useEffect(() => {
    api.getProject(projectId).then((data) => {
      setOrgModalProject({ id: projectId, name: data.project.name });
    });
  }, [projectId]);

  if (!orgModalProject) {
    return null;
  }

  return (
    <OrgSettingsModal
      projectId={orgModalProject.id}
      projectName={orgModalProject.name}
      overview={null}
      onClose={onClose}
      onSaved={onClose}
    />
  );
}
