<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import type {
  Attachment,
  Project,
  Task,
} from "../../bindings/github.com/jesusuzcategui/troopa/internal/store/models";
import * as AppService from "../../bindings/github.com/jesusuzcategui/troopa/internal/app/service";
import * as DesktopService from "../../bindings/github.com/jesusuzcategui/troopa/cmd/troopa-desktop/desktopservice";
import { AGENTS, PRIORITIES, STATUSES, SYNC_MS } from "../constants";
import { t, type Lang } from "../i18n";
import ConfirmDialog from "./ConfirmDialog.vue";
import TaskFormModal from "./TaskFormModal.vue";

const props = defineProps<{
  project: Project;
  lang: Lang;
}>();

const emit = defineEmits<{
  back: [];
  notice: [message: string];
  error: [message: string];
  openRun: [runId: number, task: Task];
  openPlans: [];
}>();

const tasks = ref<Task[]>([]);
const attachments = ref<Attachment[]>([]);
const loading = ref(true);
const query = ref("");
const selectedId = ref<number | null>(null);
const stamp = ref("");
const dragOverStatus = ref("");

const formOpen = ref(false);
const editingTask = ref<Task | null>(null);

const deleteOpen = ref(false);
const detachOpen = ref(false);
const launchOpen = ref(false);
const pendingDetachId = ref(0);
const pendingLaunchId = ref(0);
const pendingLaunchStatus = ref("ready");

let syncTimer: ReturnType<typeof setInterval> | null = null;
let dragTaskId = 0;

const selected = computed(() => tasks.value.find((t) => t.id === selectedId.value) ?? null);

const columns = computed(() => {
  const q = query.value.trim().toLowerCase();
  const filtered = q
    ? tasks.value.filter(
        (t) =>
          t.title.toLowerCase().includes(q) ||
          t.description.toLowerCase().includes(q) ||
          t.agent.toLowerCase().includes(q)
      )
    : tasks.value;
  return STATUSES.map((status) => ({
    status,
    tasks: filtered.filter((t) => t.status === status).sort((a, b) => a.position - b.position),
  }));
});

async function loadTasks(selectId?: number | null) {
  const keep = selectId === undefined ? selectedId.value : selectId;
  try {
    const list = await AppService.ListTasks(props.project.id, "");
    tasks.value = list ?? [];
    if (keep != null && tasks.value.some((t) => t.id === keep)) {
      selectedId.value = keep;
    } else if (selectedId.value != null && !tasks.value.some((t) => t.id === selectedId.value)) {
      selectedId.value = null;
      attachments.value = [];
    }
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  } finally {
    loading.value = false;
  }
}

async function loadAttachments(taskId: number) {
  try {
    const list = await AppService.ListAttachments(taskId);
    attachments.value = list ?? [];
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function selectTask(task: Task) {
  selectedId.value = task.id;
  await loadAttachments(task.id);
  try {
    const run = await AppService.LatestRun(task.id);
    emit("openRun", run.id, task);
  } catch {
    // A task without a run keeps opening in the detail panel.
  }
}

async function syncFromStore() {
  try {
    const next = await AppService.ChangeStamp();
    if (!stamp.value) {
      stamp.value = next;
      return;
    }
    if (next !== stamp.value) {
      stamp.value = next;
      await loadTasks();
      if (selectedId.value != null) {
        await loadAttachments(selectedId.value);
      }
    }
  } catch {
    /* ignore transient poll errors */
  }
}

function openCreate() {
  editingTask.value = null;
  formOpen.value = true;
}

function openEdit(task?: Task | null) {
  const target = task ?? selected.value;
  if (!target) return;
  editingTask.value = target;
  formOpen.value = true;
}

async function onSaved(task: Task, isNew: boolean) {
  formOpen.value = false;
  editingTask.value = null;
  emit("notice", `${t(props.lang, isNew ? "Created:" : "Saved:")} ${task.title}`);
  stamp.value = await AppService.ChangeStamp();
  await loadTasks(task.id);
  await loadAttachments(task.id);
}

function askDelete(task?: Task | null) {
  const target = task ?? selected.value;
  if (!target) return;
  selectedId.value = target.id;
  deleteOpen.value = true;
}

async function confirmDelete() {
  const id = selectedId.value;
  deleteOpen.value = false;
  if (id == null) return;
  try {
    await AppService.DeleteTask(id);
    emit("notice", t(props.lang, "Task deleted"));
    selectedId.value = null;
    attachments.value = [];
    stamp.value = await AppService.ChangeStamp();
    await loadTasks(null);
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function setPriority(task: Task, priority: string) {
  try {
    await AppService.UpdateTask({ ...task, priority });
    stamp.value = await AppService.ChangeStamp();
    await loadTasks(task.id);
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function setAgent(task: Task, agent: string) {
  try {
    await AppService.UpdateTask({ ...task, agent });
    stamp.value = await AppService.ChangeStamp();
    await loadTasks(task.id);
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function setStatus(task: Task, status: string) {
  if (status === task.status) return;
  if (status === "ready" && task.agent && task.agent !== "me" && task.status !== "ready") {
    pendingLaunchId.value = task.id;
    pendingLaunchStatus.value = status;
    launchOpen.value = true;
    return;
  }
  await applyMove(task.id, status);
}

async function applyMove(id: number, status: string) {
  try {
    await AppService.MoveTask(id, status);
    emit("notice", `${t(props.lang, "Moved to ")}${status}`);
    stamp.value = await AppService.ChangeStamp();
    await loadTasks(id);
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function confirmLaunch() {
  const id = pendingLaunchId.value;
  launchOpen.value = false;
  if (!id) return;
  try {
    await AppService.LaunchTask(id);
    emit("notice", t(props.lang, "Agent launched"));
    stamp.value = await AppService.ChangeStamp();
    await loadTasks(id);
  } catch (err) {
    // Fall back to a plain status move if launch fails (e.g. agent missing).
    try {
      await applyMove(id, pendingLaunchStatus.value);
    } catch {
      /* applyMove already emitted */
    }
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

function declineLaunch() {
  launchOpen.value = false;
  pendingLaunchId.value = 0;
  emit("notice", t(props.lang, "Not launched"));
}

function onDragStart(task: Task, ev: DragEvent) {
  dragTaskId = task.id;
  if (ev.dataTransfer) {
    ev.dataTransfer.effectAllowed = "move";
    ev.dataTransfer.setData("text/plain", String(task.id));
  }
}

function onDragOver(status: string, ev: DragEvent) {
  ev.preventDefault();
  dragOverStatus.value = status;
  if (ev.dataTransfer) ev.dataTransfer.dropEffect = "move";
}

function onDragLeave(status: string) {
  if (dragOverStatus.value === status) dragOverStatus.value = "";
}

async function onDrop(status: string, ev: DragEvent) {
  ev.preventDefault();
  dragOverStatus.value = "";
  const raw = ev.dataTransfer?.getData("text/plain") || String(dragTaskId);
  const id = Number(raw);
  dragTaskId = 0;
  const task = tasks.value.find((t) => t.id === id);
  if (!task || task.status === status) return;
  await selectTask(task);
  await setStatus(task, status);
}

async function attachFile() {
  if (!selected.value) return;
  try {
    const path = await DesktopService.PickFile(t(props.lang, "Attach a file"));
    if (!path) return;
    await AppService.AttachTaskFile(selected.value.id, path);
    emit("notice", `${t(props.lang, "Attached:")} ${path.split(/[/\\]/).pop()}`);
    stamp.value = await AppService.ChangeStamp();
    await loadAttachments(selected.value.id);
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

function askDetach(att: Attachment) {
  pendingDetachId.value = att.id;
  detachOpen.value = true;
}

async function confirmDetach() {
  const id = pendingDetachId.value;
  detachOpen.value = false;
  if (!id || selectedId.value == null) return;
  try {
    await AppService.DeleteAttachment(id);
    emit("notice", t(props.lang, "Attachment removed"));
    stamp.value = await AppService.ChangeStamp();
    await loadAttachments(selectedId.value);
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

const deleteMessage = computed(() => {
  const task = selected.value;
  if (!task) return "";
  return t(props.lang, "Delete task «%s»? Its runs, comments and attachments go with it.").replace(
    "%s",
    task.title
  );
});

const detachMessage = computed(() => {
  const att = attachments.value.find((a) => a.id === pendingDetachId.value);
  if (!att) return "";
  return t(props.lang, "Remove attachment «%s»?").replace("%s", att.name);
});

const launchMessage = computed(() => {
  const task = tasks.value.find((x) => x.id === pendingLaunchId.value);
  if (!task) return "";
  return t(props.lang, "Launch %s on «%s»?")
    .replace("%s", task.agent)
    .replace("%s", task.title);
});

watch(
  () => props.project.id,
  async () => {
    loading.value = true;
    selectedId.value = null;
    attachments.value = [];
    stamp.value = "";
    await loadTasks(null);
    stamp.value = await AppService.ChangeStamp();
  }
);

onMounted(async () => {
  await loadTasks(null);
  stamp.value = await AppService.ChangeStamp();
  syncTimer = setInterval(() => {
    void syncFromStore();
  }, SYNC_MS);
});

onUnmounted(() => {
  if (syncTimer) clearInterval(syncTimer);
});
</script>

<template>
  <div class="board-shell">
    <div class="board-head">
      <div class="board-head-left">
        <button class="btn sm ghost" type="button" @click="emit('back')">
          ← {{ t(lang, "Projects") }}
        </button>
        <div>
          <h1 class="page-title">{{ project.name }}</h1>
          <p class="page-subtitle">{{ t(lang, "Task board — same store as the TUI.") }}</p>
        </div>
      </div>
      <button class="btn primary" type="button" @click="openCreate">
        + {{ t(lang, "New task") }}
      </button>
      <button class="btn" type="button" @click="emit('openPlans')">
        {{ t(lang, "Plans") }}
      </button>
    </div>

    <div class="toolbar">
      <input
        v-model="query"
        class="search-input"
        type="search"
        :placeholder="t(lang, 'Search tasks…')"
        :aria-label="t(lang, 'Search tasks…')"
      />
    </div>

    <div v-if="loading" class="empty-state">
      <p>{{ t(lang, "Loading tasks…") }}</p>
    </div>

    <div v-else class="board-layout">
      <div class="kanban" role="list" :aria-label="t(lang, 'Task board')">
        <section
          v-for="col in columns"
          :key="col.status"
          class="kanban-col"
          :class="{ 'drag-over': dragOverStatus === col.status }"
          @dragover="onDragOver(col.status, $event)"
          @dragleave="onDragLeave(col.status)"
          @drop="onDrop(col.status, $event)"
        >
          <header class="kanban-col-head">
            <h2>{{ col.status }}</h2>
            <span class="kanban-count">{{ col.tasks.length }}</span>
          </header>
          <div class="kanban-cards">
            <article
              v-for="task in col.tasks"
              :key="task.id"
              class="task-card"
              :class="{ selected: selectedId === task.id }"
              draggable="true"
              role="listitem"
              tabindex="0"
              @click="selectTask(task)"
              @dblclick="openEdit(task)"
              @dragstart="onDragStart(task, $event)"
              @keydown.enter="selectTask(task)"
            >
              <h3 class="task-card-title">{{ task.title }}</h3>
              <div class="task-card-meta">
                <span class="badge" :class="{ accent: task.priority === 'high', peach: task.priority === 'medium' }">
                  {{ task.priority }}
                </span>
                <span class="badge">{{ task.agent }}</span>
              </div>
            </article>
            <p v-if="col.tasks.length === 0" class="kanban-empty">{{ t(lang, "empty") }}</p>
          </div>
        </section>
      </div>

      <aside class="task-panel" :aria-label="t(lang, 'Task panel')">
        <template v-if="selected">
          <div class="panel-head">
            <h2>{{ selected.title }}</h2>
            <div class="panel-actions">
              <button class="btn sm" type="button" @click="openEdit(selected)">
                {{ t(lang, "Edit") }}
              </button>
              <button class="btn sm ghost danger" type="button" @click="askDelete(selected)">
                {{ t(lang, "Delete") }}
              </button>
            </div>
          </div>

          <div class="panel-fields">
            <div class="field">
              <label>{{ t(lang, "Status") }}</label>
              <select
                :value="selected.status"
                @change="setStatus(selected, ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="s in STATUSES" :key="s" :value="s">{{ s }}</option>
              </select>
            </div>
            <div class="field">
              <label>{{ t(lang, "Priority") }}</label>
              <select
                :value="selected.priority"
                @change="setPriority(selected, ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="p in PRIORITIES" :key="p" :value="p">{{ p }}</option>
              </select>
            </div>
            <div class="field">
              <label>{{ t(lang, "Agent") }}</label>
              <select
                :value="selected.agent"
                @change="setAgent(selected, ($event.target as HTMLSelectElement).value)"
              >
                <option v-for="a in AGENTS" :key="a" :value="a">{{ a }}</option>
              </select>
            </div>
            <div class="field">
              <label>{{ t(lang, "Type") }}</label>
              <input :value="selected.type" type="text" readonly />
            </div>
            <div class="field">
              <label>{{ t(lang, "Mode") }}</label>
              <input :value="selected.mode" type="text" readonly />
            </div>
            <div v-if="selected.model" class="field">
              <label>{{ t(lang, "Model") }}</label>
              <input :value="selected.model" type="text" readonly />
            </div>
          </div>

          <div class="panel-section">
            <h3>{{ t(lang, "Description") }}</h3>
            <pre class="panel-md">{{ selected.description || t(lang, "(none)") }}</pre>
          </div>

          <div class="panel-section">
            <h3>{{ t(lang, "Acceptance criteria") }}</h3>
            <pre class="panel-md">{{ selected.acceptance_criteria || t(lang, "(none)") }}</pre>
          </div>

          <div class="panel-section">
            <div class="panel-section-head">
              <h3>{{ t(lang, "Attachments") }}</h3>
              <button class="btn sm" type="button" @click="attachFile">
                {{ t(lang, "Attach") }}
              </button>
            </div>
            <ul v-if="attachments.length" class="attach-list">
              <li v-for="att in attachments" :key="att.id">
                <div>
                  <strong>{{ att.name }}</strong>
                  <span class="attach-meta">{{ formatSize(att.size) }} · {{ att.mime || "file" }}</span>
                </div>
                <button class="btn sm ghost danger" type="button" @click="askDetach(att)">
                  {{ t(lang, "Remove") }}
                </button>
              </li>
            </ul>
            <p v-else class="panel-muted">{{ t(lang, "(none yet)") }}</p>
          </div>
        </template>
        <div v-else class="panel-empty">
          <p>{{ t(lang, "Select a task to see its details.") }}</p>
        </div>
      </aside>
    </div>

    <TaskFormModal
      :open="formOpen"
      :project-id="project.id"
      :task="editingTask"
      :lang="lang"
      @close="formOpen = false"
      @saved="onSaved"
    />

    <ConfirmDialog
      :open="deleteOpen"
      :title="t(lang, 'Delete task')"
      :message="deleteMessage"
      :confirm-label="t(lang, 'Yes, delete')"
      danger
      :lang="lang"
      @close="deleteOpen = false"
      @confirm="confirmDelete"
    />

    <ConfirmDialog
      :open="detachOpen"
      :title="t(lang, 'Remove attachment')"
      :message="detachMessage"
      :confirm-label="t(lang, 'Yes, remove')"
      danger
      :lang="lang"
      @close="detachOpen = false"
      @confirm="confirmDetach"
    />

    <ConfirmDialog
      :open="launchOpen"
      :title="t(lang, 'Launch agent')"
      :message="launchMessage"
      :confirm-label="t(lang, 'Yes, launch')"
      :lang="lang"
      @close="declineLaunch"
      @confirm="confirmLaunch"
    />
  </div>
</template>
