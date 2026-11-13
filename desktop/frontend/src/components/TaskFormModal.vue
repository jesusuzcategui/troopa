<script setup lang="ts">
import { ref, watch } from "vue";
import type { Task } from "../../bindings/github.com/jesusuzcategui/troopa/internal/store/models";
import * as AppService from "../../bindings/github.com/jesusuzcategui/troopa/internal/app/service";
import { AGENTS, MODES, PRIORITIES, STATUSES, TYPES } from "../constants";
import { t, type Lang } from "../i18n";

const props = defineProps<{
  open: boolean;
  projectId: number;
  task: Task | null;
  lang: Lang;
}>();

const emit = defineEmits<{
  close: [];
  saved: [task: Task, isNew: boolean];
}>();

const title = ref("");
const description = ref("");
const criteria = ref("");
const status = ref("inbox");
const priority = ref("medium");
const agent = ref("claude");
const model = ref("");
const mode = ref("yolo");
const type = ref("feature");

const saving = ref(false);
const error = ref("");

watch(
  () => [props.open, props.task] as const,
  ([isOpen, task]) => {
    if (!isOpen) return;
    error.value = "";
    saving.value = false;
    if (task) {
      title.value = task.title;
      description.value = task.description;
      criteria.value = task.acceptance_criteria;
      status.value = task.status;
      priority.value = task.priority;
      agent.value = task.agent;
      model.value = task.model;
      mode.value = task.mode;
      type.value = task.type;
    } else {
      title.value = "";
      description.value = "";
      criteria.value = "";
      status.value = "inbox";
      priority.value = "medium";
      agent.value = "claude";
      model.value = "";
      mode.value = "yolo";
      type.value = "feature";
    }
  },
  { immediate: true }
);

function buildPayload(): Task {
  const now = new Date().toISOString();
  return {
    id: props.task?.id ?? 0,
    project_id: props.projectId,
    title: title.value.trim(),
    description: description.value,
    acceptance_criteria: criteria.value,
    status: status.value,
    priority: priority.value,
    agent: agent.value,
    model: model.value.trim(),
    mode: mode.value,
    commits: props.task?.commits || "auto",
    type: type.value,
    position: props.task?.position ?? 0,
    plan_id: props.task?.plan_id,
    plan_step: props.task?.plan_step,
    created_at: props.task?.created_at || now,
    updated_at: now,
  };
}

async function submit() {
  error.value = "";
  const payload = buildPayload();
  if (!payload.title) {
    error.value = t(props.lang, "Title is required");
    return;
  }
  saving.value = true;
  try {
    if (payload.id === 0) {
      const created = await AppService.CreateTask(payload);
      emit("saved", created, true);
    } else {
      await AppService.UpdateTask(payload);
      const updated = await AppService.GetTask(payload.id);
      emit("saved", updated, false);
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
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
    :aria-label="task ? t(lang, 'Edit task') : t(lang, 'New task')"
    @click.self="!saving && emit('close')"
    @keydown.esc="!saving && emit('close')"
  >
    <div class="modal task-form-modal">
      <div class="modal-head">
        <h2>{{ task ? t(lang, "Edit task") : t(lang, "New task") }}</h2>
        <button class="btn sm ghost" type="button" :disabled="saving" @click="emit('close')">
          {{ t(lang, "Close") }}
        </button>
      </div>

      <form id="task-form" class="modal-body form-grid" @submit.prevent="submit">
        <div v-if="error" class="alert error" role="alert">{{ error }}</div>

        <div class="field">
          <label for="task-title">{{ t(lang, "Title") }}</label>
          <input id="task-title" v-model="title" type="text" required autofocus />
        </div>

        <div class="form-row-2">
          <div class="field">
            <label for="task-status">{{ t(lang, "Status") }}</label>
            <select id="task-status" v-model="status">
              <option v-for="s in STATUSES" :key="s" :value="s">{{ t(lang, s) }}</option>
            </select>
          </div>
          <div class="field">
            <label for="task-priority">{{ t(lang, "Priority") }}</label>
            <select id="task-priority" v-model="priority">
              <option v-for="p in PRIORITIES" :key="p" :value="p">{{ t(lang, p) }}</option>
            </select>
          </div>
        </div>

        <div class="form-row-2">
          <div class="field">
            <label for="task-agent">{{ t(lang, "Agent") }}</label>
            <select id="task-agent" v-model="agent">
              <option v-for="a in AGENTS" :key="a" :value="a">{{ a }}</option>
            </select>
          </div>
          <div class="field">
            <label for="task-type">{{ t(lang, "Type") }}</label>
            <select id="task-type" v-model="type">
              <option v-for="ty in TYPES" :key="ty" :value="ty">{{ t(lang, ty) }}</option>
            </select>
          </div>
        </div>

        <div class="form-row-2">
          <div class="field">
            <label for="task-mode">{{ t(lang, "Mode") }}</label>
            <select id="task-mode" v-model="mode">
              <option v-for="m in MODES" :key="m" :value="m">{{ t(lang, m) }}</option>
            </select>
          </div>
          <div class="field">
            <label for="task-model">{{ t(lang, "Model") }}</label>
            <input id="task-model" v-model="model" type="text" :placeholder="t(lang, 'Optional')" />
          </div>
        </div>

        <div class="field">
          <label for="task-desc">{{ t(lang, "Description") }}</label>
          <textarea id="task-desc" v-model="description" rows="4"></textarea>
        </div>

        <div class="field">
          <label for="task-criteria">{{ t(lang, "Acceptance criteria") }}</label>
          <textarea id="task-criteria" v-model="criteria" rows="3"></textarea>
        </div>
      </form>
      <div class="modal-actions">
        <button class="btn" type="button" :disabled="saving" @click="emit('close')">
          {{ t(lang, "Cancel") }}
        </button>
        <button class="btn primary" type="submit" form="task-form" :disabled="saving">
          {{ saving ? t(lang, "Saving…") : task ? t(lang, "Save") : t(lang, "Create") }}
        </button>
      </div>
    </div>
  </div>
</template>
