<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from "vue";
import type { Client, Profile, Project } from "../bindings/github.com/jesusuzcategui/troopa/internal/store/models";
import type { ProjectSummary } from "../bindings/github.com/jesusuzcategui/troopa/internal/app/models";
import type { Palette } from "../bindings/github.com/jesusuzcategui/troopa/internal/theme/models";
import * as AppService from "../bindings/github.com/jesusuzcategui/troopa/internal/app/service";
import * as DesktopService from "../bindings/github.com/jesusuzcategui/troopa/cmd/troopa-desktop/desktopservice";
import TroopaLogo from "./components/TroopaLogo.vue";
import ProjectFormModal from "./components/ProjectFormModal.vue";
import TaskBoard from "./components/TaskBoard.vue";
import PlansView from "./components/PlansView.vue";
import RunView from "./components/RunView.vue";
import SettingsModal from "./components/SettingsModal.vue";
import ClientsModal from "./components/ClientsModal.vue";
import { t, type Lang } from "./i18n";
import { applyPalette, findTheme } from "./theme";

const summaries = ref<ProjectSummary[]>([]);
const clients = ref<Client[]>([]);
const profiles = ref<Profile[]>([]);
const accounts = ref<string[]>([]);

const loading = ref(true);
const error = ref("");
const notice = ref("");
const query = ref("");
const showArchived = ref(false);

const modalOpen = ref(false);
const editingProject = ref<Project | null>(null);
const openProject = ref<Project | null>(null);
const openRun = ref<{ id: number; title: string } | null>(null);
const plansOpen = ref(false);
const settingsOpen = ref(false);
const clientsOpen = ref(false);
const projectForm = ref<InstanceType<typeof ProjectFormModal> | null>(null);
const collapsedClients = ref<Record<string, boolean>>({});
const desktopSettings = ref<Awaited<ReturnType<typeof DesktopService.GetSettings>> | null>(null);

const lang = ref<Lang>("en");
const themes = ref<Palette[]>([]);
const activeTheme = ref("dark");
const headerMenuOpen = ref(false);
const header = ref<HTMLElement | null>(null);
const headerTools = ref<HTMLElement | null>(null);
const headerMenuTop = ref(0);

function updateHeaderMenuTop() {
  if (!header.value) return;
  headerMenuTop.value = Math.round(header.value.getBoundingClientRect().bottom);
}

function onHeaderMenuResize() {
  if (window.innerWidth >= 768) {
    headerMenuOpen.value = false;
    return;
  }
  if (headerMenuOpen.value) updateHeaderMenuTop();
}

async function toggleHeaderMenu() {
  headerMenuOpen.value = !headerMenuOpen.value;
  if (headerMenuOpen.value) {
    updateHeaderMenuTop();
    await nextTick();
    updateHeaderMenuTop();
  }
}

function closeHeaderMenuOnOutsideClick(event: PointerEvent) {
  if (!headerMenuOpen.value || !(event.target instanceof Node)) return;
  if (header.value?.contains(event.target)) return;
  if (headerTools.value?.contains(event.target)) return;
  headerMenuOpen.value = false;
}

function applyThemeByName(name: string) {
  const match = findTheme(themes.value, name);
  if (match) {
    activeTheme.value = match.name;
    applyPalette(match);
  }
}

function applyLang(next: Lang) {
  lang.value = next;
  document.documentElement.lang = next;
}

async function toggleLang() {
  const next: Lang = lang.value === "en" ? "es" : "en";
  applyLang(next);
  try {
    const current = desktopSettings.value ?? (await DesktopService.GetSettings());
    await DesktopService.SaveSettings({
      language: next,
      default_agent: current.default_agent,
      worktrees: current.worktrees,
      terminal: current.terminal,
      setup_done: current.setup_done,
    });
    desktopSettings.value = await DesktopService.GetSettings();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

async function openSettings() {
  try {
    const [themeList, settings] = await Promise.all([
      AppService.ListThemes(),
      DesktopService.GetSettings(),
    ]);
    themes.value = themeList ?? [];
    desktopSettings.value = settings;
  } catch {
    /* open with cached settings */
  }
  settingsOpen.value = true;
}

async function settingsSaved(settings: Awaited<ReturnType<typeof DesktopService.GetSettings>>) {
  desktopSettings.value = settings;
  applyLang(settings.language === "es" ? "es" : "en");
  applyThemeByName(settings.theme || settings.theme_mode || "dark");
  await loadData();
}

const profileNames = computed(() => {
  const map = new Map<number, string>();
  for (const p of profiles.value) {
    map.set(p.id, p.name);
  }
  return map;
});

const filteredSummaries = computed(() => {
  const q = query.value.trim().toLowerCase();
  return summaries.value.filter(({ project: p }) => {
    if (!showArchived.value && p.archived) return false;
    if (!q) return true;
    return (
      p.name.toLowerCase().includes(q) ||
      p.slug.toLowerCase().includes(q) ||
      p.repo_path.toLowerCase().includes(q) ||
      p.stack.toLowerCase().includes(q) ||
      p.description.toLowerCase().includes(q)
    );
  });
});

const groupedSummaries = computed(() => {
  const groups = new Map<number, { client: Client | null; summaries: ProjectSummary[] }>();
  for (const summary of filteredSummaries.value) {
    const client = clients.value.find((item) => item.id === summary.project.client_id) ?? null;
    const key = client?.id ?? 0;
    const group = groups.get(key) ?? { client, summaries: [] };
    group.summaries.push(summary);
    groups.set(key, group);
  }
  return [...groups.values()].sort((a, b) => (a.client?.name ?? t(lang.value, "No client")).localeCompare(b.client?.name ?? t(lang.value, "No client")));
});

async function loadData() {
  loading.value = true;
  error.value = "";
  try {
    const [summaryList, clientList, profileList, accountList] = await Promise.all([
      AppService.ListProjectSummaries(true),
      AppService.ListClients(true),
      AppService.ListProfiles(),
      AppService.ListAccounts(),
    ]);
    summaries.value = summaryList ?? [];
    clients.value = clientList ?? [];
    profiles.value = profileList ?? [];
    accounts.value = accountList ?? [];
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editingProject.value = null;
  modalOpen.value = true;
}

function openEdit(proj: Project) {
  editingProject.value = proj;
  modalOpen.value = true;
}

function openBoard(proj: Project) {
  error.value = "";
  notice.value = "";
  openProject.value = proj;
}

function closeBoard() {
  openRun.value = null;
  plansOpen.value = false;
  openProject.value = null;
  void loadData();
}

function showRun(id: number, task: { title: string }) {
  openRun.value = { id, title: task.title };
}

function closeRun() {
  openRun.value = null;
}

function openPlans() {
  plansOpen.value = true;
  openRun.value = null;
}

async function onSaved(proj: Project, isNew: boolean) {
  modalOpen.value = false;
  editingProject.value = null;
  notice.value = `${t(lang.value, isNew ? "Created:" : "Saved:")} ${proj.name}`;
  await loadData();
}

async function archiveProject(proj: Project) {
  error.value = "";
  try {
    await AppService.ArchiveProject(proj.id);
    notice.value = `${t(lang.value, "Archived:")} ${proj.name}`;
    await loadData();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}

function openClients() {
  clientsOpen.value = true;
}

async function clientChanged(client: Client, isNew: boolean) {
  notice.value = `${t(lang.value, isNew ? "Created:" : "Saved:")} ${client.name}`;
  await loadData();
  if (isNew) projectForm.value?.selectClient(client.id);
}

function lastActivity(value: string) {
  return value ? new Date(value).toLocaleDateString(lang.value === "es" ? "es" : "en", { month: "short", day: "numeric" }) : "—";
}

onMounted(async () => {
  document.addEventListener("pointerdown", closeHeaderMenuOnOutsideClick);
  window.addEventListener("resize", onHeaderMenuResize);
  try {
    const [themeList, settings] = await Promise.all([
      AppService.ListThemes(),
      DesktopService.GetSettings(),
    ]);

    themes.value = themeList ?? [];
    desktopSettings.value = settings;
    applyThemeByName(settings.theme || settings.theme_mode || "dark");

    // Language always comes from config.toml (same store as the TUI).
    applyLang(settings.language === "es" ? "es" : "en");
  } catch {
    const prefersLight = window.matchMedia("(prefers-color-scheme: light)").matches;
    document.documentElement.dataset.theme = prefersLight ? "light" : "dark";
  }

  if (desktopSettings.value && !desktopSettings.value.setup_done) settingsOpen.value = true;

  await loadData();
});

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", closeHeaderMenuOnOutsideClick);
  window.removeEventListener("resize", onHeaderMenuResize);
});
</script>

<template>
  <div class="app-shell" :class="{ 'board-mode': !!openProject }">
    <header ref="header" class="topbar">
      <div class="topbar-inner" :class="{ wide: !!openProject }">
        <div class="brand">
          <TroopaLogo />
        </div>
        <button
          class="tool-btn menu-toggle"
          type="button"
          :aria-expanded="headerMenuOpen"
          aria-controls="topbar-tools"
          @click="toggleHeaderMenu"
        >
          {{ t(lang, headerMenuOpen ? "Close" : "Menu") }}
        </button>
        <Teleport to="body" :disabled="!headerMenuOpen">
          <div
            id="topbar-tools"
            ref="headerTools"
            class="topbar-tools"
            :class="{ open: headerMenuOpen }"
            :style="headerMenuOpen ? { top: `${headerMenuTop}px` } : undefined"
          >
            <button
              class="tool-btn"
              type="button"
              :title="t(lang, 'Switch language')"
              @click="toggleLang(); headerMenuOpen = false"
            >
              {{ lang === "en" ? "ES" : "EN" }}
            </button>
            <button class="tool-btn" type="button" :title="t(lang, 'Settings')" @click="openSettings(); headerMenuOpen = false">{{ t(lang, "Settings") }}</button>
          </div>
        </Teleport>
      </div>
    </header>

    <main class="main" :class="{ wide: !!openProject }">
      <div v-if="error" class="alert error" role="alert">{{ error }}</div>
      <div v-if="notice" class="alert ok" role="status">{{ notice }}</div>

      <RunView
        v-if="openRun"
        :run-id="openRun.id"
        :task-title="openRun.title"
        :lang="lang"
        @back="closeRun"
        @notice="notice = $event"
        @error="error = $event"
      />

      <PlansView
        v-else-if="openProject && plansOpen"
        :project="openProject"
        :lang="lang"
        @back="plansOpen = false"
        @notice="notice = $event"
        @error="error = $event"
        @open-run="showRun"
      />

      <TaskBoard
        v-else-if="openProject"
        :project="openProject"
        :lang="lang"
        @back="closeBoard"
        @notice="notice = $event"
        @error="error = $event"
        @open-run="showRun"
        @open-plans="openPlans"
        @open-clients="openClients"
      />

      <template v-else>
        <div class="page-head">
          <div>
            <h1 class="page-title">{{ t(lang, "Projects") }}</h1>
            <p class="page-subtitle">{{ t(lang, "Manage repositories shared with your Troopa TUI and CLI.") }}</p>
          </div>
          <div class="page-actions">
            <button class="btn" type="button" @click="openClients">{{ t(lang, "Clients") }}</button>
            <button class="btn primary" type="button" @click="openCreate">+ {{ t(lang, "New project") }}</button>
          </div>
        </div>

        <div class="toolbar">
          <input
            v-model="query"
            class="search-input"
            type="search"
            :placeholder="t(lang, 'Search projects…')"
            :aria-label="t(lang, 'Search projects…')"
          />
          <label class="toggle-label">
            <input v-model="showArchived" type="checkbox" />
            {{ t(lang, "Show archived") }}
          </label>
        </div>

        <div v-if="loading" class="empty-state">
          <p>{{ t(lang, "Loading projects…") }}</p>
        </div>

        <div v-else-if="summaries.length === 0" class="empty-state">
          <h2>{{ t(lang, "No projects yet") }}</h2>
          <p>{{ t(lang, "Create your first project to point Troopa at a git repository.") }}</p>
          <button class="btn primary" type="button" @click="openCreate">
            + {{ t(lang, "New project") }}
          </button>
        </div>

        <div v-else-if="filteredSummaries.length === 0" class="empty-state">
          <h2>{{ t(lang, "No matching projects") }}</h2>
          <p>{{ t(lang, "Try another search or show archived projects.") }}</p>
        </div>

        <section v-else class="client-groups" :aria-label="t(lang, 'Projects')">
          <section v-for="group in groupedSummaries" :key="group.client?.id ?? 0" class="client-group">
            <button class="client-group-head" type="button" :aria-expanded="!collapsedClients[String(group.client?.id ?? 0)]" @click="collapsedClients[String(group.client?.id ?? 0)] = !collapsedClients[String(group.client?.id ?? 0)]">
              <span>{{ group.client?.name ?? t(lang, "No client") }}</span>
              <span class="client-group-toggle">{{ t(lang, collapsedClients[String(group.client?.id ?? 0)] ? "Expand" : "Collapse") }}</span>
              <span class="badge">{{ group.summaries.length }}</span>
            </button>
            <div v-if="!collapsedClients[String(group.client?.id ?? 0)]" class="project-list">
          <article v-for="summary in group.summaries" :key="summary.project.id" class="project-card" :class="{ archived: summary.project.archived }">
            <template v-for="proj in [summary.project]" :key="proj.id">
            <div class="project-top">
              <div>
                <h2 class="project-name">
                  <button class="linkish" type="button" @click="openBoard(proj)">
                    {{ proj.name }}
                  </button>
                </h2>
                <span class="project-slug">{{ proj.slug }}</span>
              </div>
              <span v-if="proj.archived" class="badge">{{ t(lang, "Archived") }}</span>
            </div>

            <div class="project-path" :title="proj.repo_path">{{ proj.repo_path }}</div>

            <p v-if="proj.description" class="project-desc">{{ proj.description }}</p>

            <div class="badges" v-if="proj.stack || proj.account || proj.profile_id">
              <span v-if="proj.stack" class="badge accent">{{ proj.stack }}</span>
              <span v-if="proj.account" class="badge peach">@{{ proj.account }}</span>
              <span v-if="proj.profile_id && profileNames.get(proj.profile_id)" class="badge">
                {{ profileNames.get(proj.profile_id) }}
              </span>
            </div>

            <div class="project-summary">
              <span v-for="(count, status) in summary.tasks" :key="status">{{ t(lang, status) }} {{ count }}</span>
              <span>{{ t(lang, "Active plans") }}: {{ summary.active_plans }}</span>
              <span>{{ t(lang, "Last activity") }}: {{ lastActivity(summary.last_activity) }}</span>
            </div>

            <div class="project-actions">
              <button
                v-if="!proj.archived"
                class="btn sm ghost danger"
                type="button"
                @click="archiveProject(proj)"
              >
                {{ t(lang, "Archive") }}
              </button>
              <button class="btn sm" type="button" @click="openEdit(proj)">
                {{ t(lang, "Edit") }}
              </button>
              <button class="btn sm primary" type="button" @click="openBoard(proj)">
                {{ t(lang, "Open board") }}
              </button>
            </div>
            </template>
          </article>
            </div>
          </section>
        </section>
      </template>
    </main>

    <ProjectFormModal
      ref="projectForm"
      :open="modalOpen"
      :project="editingProject"
      :clients="clients.filter((c) => !c.archived)"
      :profiles="profiles"
      :accounts="accounts"
      :lang="lang"
      @close="modalOpen = false"
      @saved="onSaved"
      @new-client="openClients"
    />
    <ClientsModal :open="clientsOpen" :clients="clients" :lang="lang" @close="clientsOpen = false" @changed="clientChanged" />
    <SettingsModal
      :open="settingsOpen"
      :first-run="!!desktopSettings && !desktopSettings.setup_done"
      :settings="desktopSettings"
      :themes="themes"
      :current-theme="activeTheme"
      :profiles="profiles"
      :lang="lang"
      @close="settingsOpen = false"
      @saved="settingsSaved"
      @preview-theme="applyThemeByName"
    />
  </div>
</template>
