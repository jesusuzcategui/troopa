<script setup lang="ts">
import { ref, watch } from "vue";
import type { Client, Profile, Project } from "../../bindings/github.com/jesusuzcategui/troopa/internal/store/models";
import * as AppService from "../../bindings/github.com/jesusuzcategui/troopa/internal/app/service";
import * as DesktopService from "../../bindings/github.com/jesusuzcategui/troopa/cmd/troopa-desktop/desktopservice";
import { t, type Lang } from "../i18n";

const props = defineProps<{
  open: boolean;
  project: Project | null;
  clients: Client[];
  profiles: Profile[];
  accounts: string[];
  lang: Lang;
}>();

const emit = defineEmits<{
  close: [];
  saved: [project: Project, isNew: boolean];
  newClient: [];
}>();

function chooseClient() {
  if (clientId.value === -1) {
    clientId.value = 0;
    emit("newClient");
  }
}

function selectClient(id: number) {
  clientId.value = id;
}

defineExpose({ selectClient });

const name = ref("");
const slug = ref("");
const repoPath = ref("");
const stack = ref("");
const description = ref("");
const clientId = ref(0);
const account = ref("");
const profileId = ref(0);

const saving = ref(false);
const error = ref("");
const confirmInit = ref(false);

watch(
  () => [props.open, props.project] as const,
  ([isOpen, proj]) => {
    if (!isOpen) return;
    error.value = "";
    confirmInit.value = false;
    saving.value = false;
    if (proj) {
      name.value = proj.name;
      slug.value = proj.slug;
      repoPath.value = proj.repo_path;
      stack.value = proj.stack;
      description.value = proj.description;
      clientId.value = proj.client_id || 0;
      account.value = proj.account || "";
      profileId.value = proj.profile_id || 0;
    } else {
      name.value = "";
      slug.value = "";
      repoPath.value = "";
      stack.value = "";
      description.value = "";
      clientId.value = 0;
      account.value = "";
      profileId.value = 0;
    }
  },
  { immediate: true }
);

function buildPayload(): Project {
  const now = new Date().toISOString();
  return {
    id: props.project?.id ?? 0,
    name: name.value.trim(),
    slug: slug.value.trim(),
    repo_path: repoPath.value.trim(),
    stack: stack.value.trim(),
    description: description.value.trim(),
    client_id: Number(clientId.value) || 0,
    account: account.value,
    profile_id: Number(profileId.value) || 0,
    archived: props.project?.archived ?? false,
    created_at: props.project?.created_at || now,
    updated_at: now,
  };
}

async function pickFolder() {
  try {
    const selected = await DesktopService.PickDirectory(t(props.lang, "Choose repository folder"));
    if (selected) {
      repoPath.value = selected;
      confirmInit.value = false;
    }
  } catch (err) {
    error.value = String(err);
  }
}

async function submit(initMissing = false) {
  error.value = "";
  const payload = buildPayload();
  if (!payload.name) {
    error.value = t(props.lang, "Name is required");
    return;
  }
  if (!payload.repo_path) {
    error.value = t(props.lang, "Repo path is required");
    return;
  }

  saving.value = true;
  try {
    if (!initMissing) {
      const exists = await AppService.PathExists(payload.repo_path);
      if (!exists) {
        confirmInit.value = true;
        saving.value = false;
        return;
      }
    } else {
      await AppService.InitRepo(payload.repo_path, payload.name);
    }

    if (payload.id === 0) {
      const created = await AppService.CreateProject(payload);
      emit("saved", created, true);
    } else {
      await AppService.UpdateProject(payload);
      const updated = await AppService.GetProject(payload.id);
      emit("saved", updated, false);
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
    confirmInit.value = false;
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <div
    v-if="open"
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    :aria-label="project ? t(lang, 'Edit project') : t(lang, 'New project')"
    @click.self="!saving && emit('close')"
    @keydown.esc="!saving && emit('close')"
  >
    <div class="modal">
      <div class="modal-head">
        <h2>{{ project ? t(lang, "Edit project") : t(lang, "New project") }}</h2>
        <button class="btn sm ghost" type="button" :disabled="saving" @click="emit('close')">
          {{ t(lang, "Close") }}
        </button>
      </div>

      <div v-if="error" class="alert error" role="alert">{{ error }}</div>

      <form class="form-grid" @submit.prevent="submit(false)">
        <div class="form-row-2">
          <div class="field">
            <label for="proj-name">{{ t(lang, "Name") }}</label>
            <input id="proj-name" v-model="name" type="text" required autofocus />
          </div>
          <div class="field">
            <label for="proj-slug">{{ t(lang, "Slug") }}</label>
            <input
              id="proj-slug"
              v-model="slug"
              type="text"
              :placeholder="t(lang, 'Optional — auto-generated from name')"
            />
          </div>
        </div>

        <div class="field">
          <label for="proj-path">{{ t(lang, "Repository path") }}</label>
          <div class="path-row">
            <input id="proj-path" v-model="repoPath" type="text" required placeholder="~/src/my-project" />
            <button class="btn" type="button" :disabled="saving" @click="pickFolder">
              {{ t(lang, "Browse…") }}
            </button>
          </div>
        </div>

        <div class="form-row-2">
          <div class="field">
            <label for="proj-stack">{{ t(lang, "Stack") }}</label>
            <input
              id="proj-stack"
              v-model="stack"
              type="text"
              :placeholder="t(lang, 'e.g. Go, Vue, TypeScript')"
            />
          </div>
          <div class="field">
            <label for="proj-client">{{ t(lang, "Client") }}</label>
            <select id="proj-client" v-model.number="clientId" @change="chooseClient">
              <option :value="0">{{ t(lang, "None") }}</option>
              <option :value="-1">{{ t(lang, "New client") }}</option>
              <option v-for="c in clients" :key="c.id" :value="c.id">{{ c.name }}</option>
            </select>
          </div>
        </div>

        <div class="form-row-2">
          <div class="field">
            <label for="proj-account">{{ t(lang, "Account") }}</label>
            <select id="proj-account" v-model="account">
              <option value="">{{ t(lang, "Default") }}</option>
              <option v-for="acc in accounts" :key="acc" :value="acc">{{ acc }}</option>
            </select>
          </div>
          <div class="field">
            <label for="proj-profile">{{ t(lang, "Profile") }}</label>
            <select id="proj-profile" v-model.number="profileId">
              <option :value="0">{{ t(lang, "Default") }}</option>
              <option v-for="pr in profiles" :key="pr.id" :value="pr.id">
                {{ pr.name }}<template v-if="pr.profession"> ({{ pr.profession }})</template>
              </option>
            </select>
          </div>
        </div>

        <div class="field">
          <label for="proj-desc">{{ t(lang, "Description") }}</label>
          <textarea
            id="proj-desc"
            v-model="description"
            :placeholder="t(lang, 'Short description')"
          ></textarea>
        </div>

        <div v-if="confirmInit" class="init-confirm">
          <p>
            {{ t(lang, "Folder does not exist yet:") }}
            <code>{{ repoPath }}</code>
            <br />
            {{ t(lang, "Create folder and initialize a git repository?") }}
          </p>
          <div class="modal-actions" style="margin-top: 8px">
            <button class="btn sm" type="button" :disabled="saving" @click="confirmInit = false">
              {{ t(lang, "Back") }}
            </button>
            <button class="btn sm primary" type="button" :disabled="saving" @click="submit(true)">
              {{ saving ? t(lang, "Saving…") : t(lang, "Create repo & save") }}
            </button>
          </div>
        </div>

        <div v-else class="modal-actions">
          <button class="btn" type="button" :disabled="saving" @click="emit('close')">
            {{ t(lang, "Cancel") }}
          </button>
          <button class="btn primary" type="submit" :disabled="saving">
            {{ saving ? t(lang, "Saving…") : project ? t(lang, "Save") : t(lang, "Create") }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
