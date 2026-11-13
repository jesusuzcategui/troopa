<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import type { Client, Profile, Project } from "../bindings/github.com/jesusuzcategui/troopa/internal/store/models";
import * as AppService from "../bindings/github.com/jesusuzcategui/troopa/internal/app/service";
import * as DesktopService from "../bindings/github.com/jesusuzcategui/troopa/cmd/troopa-desktop/desktopservice";
import TroopaLogo from "./components/TroopaLogo.vue";
import ProjectFormModal from "./components/ProjectFormModal.vue";
import TaskBoard from "./components/TaskBoard.vue";
import PlansView from "./components/PlansView.vue";
import RunView from "./components/RunView.vue";
import SettingsModal from "./components/SettingsModal.vue";
import { t, type Lang } from "./i18n";

const projects = ref<Project[]>([]);
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
const desktopSettings = ref<Awaited<ReturnType<typeof DesktopService.GetSettings>> | null>(null);

const lang = ref<Lang>("en");
const theme = ref<"light" | "dark">("dark");

function applyTheme(mode: "light" | "dark") {
  theme.value = mode;
  document.documentElement.dataset.theme = mode;
  try {
    localStorage.setItem("troopa-desktop-theme", mode);
  } catch {
    /* ignore storage errors */
  }
}

function toggleTheme() {
  applyTheme(theme.value === "dark" ? "light" : "dark");
}

function applyLang(next: Lang) {
  lang.value = next;
  document.documentElement.lang = next;
  try {
    localStorage.setItem("troopa-desktop-lang", next);
  } catch {
    /* ignore storage errors */
  }
}

function toggleLang() {
  applyLang(lang.value === "en" ? "es" : "en");
}

function openSettings() { settingsOpen.value = true; }

async function settingsSaved(settings: Awaited<ReturnType<typeof DesktopService.GetSettings>>) {
  desktopSettings.value = settings;
  applyLang(settings.language === "es" ? "es" : "en");
  await loadData();
}

const clientNames = computed(() => {
  const map = new Map<number, string>();
  for (const c of clients.value) {
    map.set(c.id, c.name);
  }
  return map;
});

const profileNames = computed(() => {
  const map = new Map<number, string>();
  for (const p of profiles.value) {
    map.set(p.id, p.name);
  }
  return map;
});

const filteredProjects = computed(() => {
  const q = query.value.trim().toLowerCase();
  return projects.value.filter((p) => {
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

async function loadData() {
  loading.value = true;
  error.value = "";
  try {
    const [projList, clientList, profileList, accountList] = await Promise.all([
      AppService.ListProjects(true),
      AppService.ListClients(true),
      AppService.ListProfiles(),
      AppService.ListAccounts(),
    ]);
    projects.value = projList ?? [];
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

onMounted(async () => {
  let savedTheme: string | null = null;
  let savedLang: string | null = null;
  try {
    savedTheme = localStorage.getItem("troopa-desktop-theme");
    savedLang = localStorage.getItem("troopa-desktop-lang");
  } catch {
    /* ignore */
  }

  try {
    const settings = await DesktopService.GetSettings();

    desktopSettings.value = settings;
    if (savedTheme === "light" || savedTheme === "dark") {
      applyTheme(savedTheme);
    } else if (settings.theme_mode === "light" || settings.theme_mode === "dark") {
      applyTheme(settings.theme_mode);
    } else {
      const prefersLight = window.matchMedia("(prefers-color-scheme: light)").matches;
      applyTheme(prefersLight ? "light" : "dark");
    }

    if (savedLang === "en" || savedLang === "es") {
      applyLang(savedLang);
    } else {
      applyLang(settings.language === "es" ? "es" : "en");
    }
  } catch {
    const prefersLight = window.matchMedia("(prefers-color-scheme: light)").matches;
    applyTheme(savedTheme === "light" ? "light" : savedTheme === "dark" ? "dark" : prefersLight ? "light" : "dark");
  }

  if (desktopSettings.value && !desktopSettings.value.setup_done) settingsOpen.value = true;

  await loadData();
});
</script>

<template>
  <div class="app-shell" :class="{ 'board-mode': !!openProject }">
    <header class="topbar">
      <div class="topbar-inner" :class="{ wide: !!openProject }">
        <div class="brand">
          <TroopaLogo />
        </div>
        <div class="topbar-tools">
          <button
            class="tool-btn"
            type="button"
            :title="t(lang, 'Switch language')"
            @click="toggleLang"
          >
            {{ lang === "en" ? "ES" : "EN" }}
          </button>
          <button
            class="tool-btn"
            type="button"
            :title="t(lang, 'Toggle light/dark theme')"
            :aria-label="t(lang, 'Toggle light/dark theme')"
            @click="toggleTheme"
          >
            ◐ {{ theme === "dark" ? "Dark" : "Light" }}
          </button>
          <button class="tool-btn" type="button" :title="t(lang, 'Settings')" @click="openSettings">{{ t(lang, "Settings") }}</button>
        </div>
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
      />

      <template v-else>
        <div class="page-head">
          <div>
            <h1 class="page-title">{{ t(lang, "Projects") }}</h1>
            <p class="page-subtitle">{{ t(lang, "Manage repositories shared with your Troopa TUI and CLI.") }}</p>
          </div>
          <button class="btn primary" type="button" @click="openCreate">
            + {{ t(lang, "New project") }}
          </button>
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

        <div v-else-if="projects.length === 0" class="empty-state">
          <h2>{{ t(lang, "No projects yet") }}</h2>
          <p>{{ t(lang, "Create your first project to point Troopa at a git repository.") }}</p>
          <button class="btn primary" type="button" @click="openCreate">
            + {{ t(lang, "New project") }}
          </button>
        </div>

        <div v-else-if="filteredProjects.length === 0" class="empty-state">
          <h2>{{ t(lang, "No matching projects") }}</h2>
          <p>{{ t(lang, "Try another search or show archived projects.") }}</p>
        </div>

        <section v-else class="project-list" aria-label="Projects">
          <article
            v-for="proj in filteredProjects"
            :key="proj.id"
            class="project-card"
            :class="{ archived: proj.archived }"
          >
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

            <div class="badges" v-if="proj.stack || proj.client_id || proj.account || proj.profile_id">
              <span v-if="proj.stack" class="badge accent">{{ proj.stack }}</span>
              <span v-if="proj.client_id && clientNames.get(proj.client_id)" class="badge">
                {{ clientNames.get(proj.client_id) }}
              </span>
              <span v-if="proj.account" class="badge peach">@{{ proj.account }}</span>
              <span v-if="proj.profile_id && profileNames.get(proj.profile_id)" class="badge">
                {{ profileNames.get(proj.profile_id) }}
              </span>
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
          </article>
        </section>
      </template>
    </main>

    <ProjectFormModal
      :open="modalOpen"
      :project="editingProject"
      :clients="clients.filter((c) => !c.archived)"
      :profiles="profiles"
      :accounts="accounts"
      :lang="lang"
      @close="modalOpen = false"
      @saved="onSaved"
    />
    <SettingsModal
      :open="settingsOpen"
      :first-run="!!desktopSettings && !desktopSettings.setup_done"
      :settings="desktopSettings"
      :profiles="profiles"
      :lang="lang"
      @close="settingsOpen = false"
      @saved="settingsSaved"
    />
  </div>
</template>
