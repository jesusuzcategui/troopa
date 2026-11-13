<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import type { Plan, Project, Task } from "../../bindings/github.com/jesusuzcategui/troopa/internal/store/models";
import * as AppService from "../../bindings/github.com/jesusuzcategui/troopa/internal/app/service";
import { AGENTS, SYNC_MS } from "../constants";
import { t, type Lang } from "../i18n";
import TaskFormModal from "./TaskFormModal.vue";

const props = defineProps<{ project: Project; lang: Lang }>();
const emit = defineEmits<{
  back: [];
  notice: [message: string];
  error: [message: string];
  openRun: [runId: number, task: Task];
}>();

const plans = ref<Plan[]>([]);
const steps = ref<Task[]>([]);
const selectedId = ref<number | null>(null);
const loading = ref(true);
const stamp = ref("");
const formOpen = ref(false);
const editing = ref<Plan | null>(null);
const title = ref("");
const goal = ref("");
const agent = ref("claude");
const model = ref("");
const stepTitle = ref("");
const editingStep = ref<Task | null>(null);
let timer: ReturnType<typeof setInterval> | null = null;

const selected = computed(() => plans.value.find((plan) => plan.id === selectedId.value) ?? null);
const done = computed(() => steps.value.filter((step) => step.status === "done").length);
const canMerge = computed(() => selected.value?.state === "done");
const canPR = computed(() => selected.value?.state === "done");
const canStart = computed(() => ["draft", "paused"].includes(selected.value?.state ?? ""));
const canPause = computed(() => selected.value?.state === "running");

async function load(select?: number | null) {
  try {
    const list = await AppService.ListPlans(props.project.id);
    plans.value = list ?? [];
    const next = select === undefined ? selectedId.value : select;
    selectedId.value = next != null && plans.value.some((plan) => plan.id === next) ? next : plans.value[0]?.id ?? null;
    if (selectedId.value != null) steps.value = (await AppService.PlanSteps(selectedId.value)) ?? [];
    else steps.value = [];
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  } finally {
    loading.value = false;
  }
}

async function syncFromStore() {
  try {
    const next = await AppService.ChangeStamp();
    if (stamp.value && next !== stamp.value) await load();
    stamp.value = next;
  } catch {
    // Ignore a transient desktop bridge failure; the next poll retries.
  }
}

function openCreate() {
  editing.value = null;
  title.value = "";
  goal.value = "";
  agent.value = "claude";
  model.value = "";
  formOpen.value = true;
}

function openEdit() {
  if (!selected.value) return;
  editing.value = selected.value;
  title.value = selected.value.title;
  goal.value = selected.value.goal;
  agent.value = selected.value.agent;
  model.value = selected.value.model;
  formOpen.value = true;
}

async function savePlan() {
  if (!title.value.trim()) return;
  try {
    if (editing.value) {
      await AppService.UpdatePlan({ ...editing.value, title: title.value.trim(), goal: goal.value.trim(), agent: agent.value, model: model.value.trim() });
      emit("notice", t(props.lang, "Plan saved"));
      await load(editing.value.id);
    } else {
      const plan = await AppService.CreatePlan({ id: 0, project_id: props.project.id, title: title.value.trim(), goal: goal.value.trim(), agent: agent.value, model: model.value.trim(), state: "draft", auto: true, auto_approve: false, base: "", created_at: "", updated_at: "" });
      emit("notice", t(props.lang, "Plan created"));
      await load(plan.id);
    }
    stamp.value = await AppService.ChangeStamp();
    formOpen.value = false;
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function updatePlan(changes: Partial<Plan>) {
  if (!selected.value) return;
  try {
    await AppService.UpdatePlan({ ...selected.value, ...changes });
    await load(selected.value.id);
    stamp.value = await AppService.ChangeStamp();
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function addStep() {
  if (!selected.value || !stepTitle.value.trim()) return;
  try {
    await AppService.AddPlanStep(selected.value.id, { id: 0, project_id: props.project.id, title: stepTitle.value.trim(), description: "", acceptance_criteria: "", status: "backlog", priority: "medium", agent: selected.value.agent, model: selected.value.model, mode: "yolo", commits: "none", type: "feature", position: 0, plan_id: selected.value.id, plan_step: 0, created_at: "", updated_at: "" });
    stepTitle.value = "";
    emit("notice", t(props.lang, "Step added"));
    await load(selected.value.id);
    stamp.value = await AppService.ChangeStamp();
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function moveStep(step: Task, direction: number) {
  try {
    await AppService.MovePlanStep(step.id, direction);
    await load(selectedId.value);
    stamp.value = await AppService.ChangeStamp();
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function openStep(step: Task) {
  try {
    const run = await AppService.LatestRun(step.id);
    emit("openRun", run.id, step);
  } catch {
    editingStep.value = step;
  }
}

async function stepSaved() {
  editingStep.value = null;
  await load(selectedId.value);
  stamp.value = await AppService.ChangeStamp();
}

async function action(label: string, call: () => Promise<void>) {
  try {
    await call();
    emit("notice", t(props.lang, label));
    await load(selectedId.value);
    stamp.value = await AppService.ChangeStamp();
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function deletePlan() {
  if (!selected.value || !window.confirm(t(props.lang, "Delete plan «%s»?").replace("%s", selected.value.title))) return;
  await action("Plan deleted", () => AppService.DeletePlan(selected.value!.id));
}

onMounted(async () => {
  await load();
  stamp.value = await AppService.ChangeStamp();
  timer = setInterval(() => void syncFromStore(), SYNC_MS);
});
onUnmounted(() => { if (timer) clearInterval(timer); });
</script>

<template>
  <section class="plans-view" :aria-label="t(lang, 'Plans')">
    <header class="board-head">
      <div class="board-head-left">
        <button class="btn sm ghost" type="button" @click="emit('back')">← {{ t(lang, "Task board") }}</button>
        <div><h1 class="page-title">{{ t(lang, "Plans") }}</h1><p class="page-subtitle">{{ project.name }}</p></div>
      </div>
      <button class="btn primary" type="button" @click="openCreate">+ {{ t(lang, "New plan") }}</button>
    </header>

    <div v-if="loading" class="empty-state"><p>{{ t(lang, "Loading plans…") }}</p></div>
    <div v-else class="plans-layout">
      <aside class="plans-list">
        <button v-for="plan in plans" :key="plan.id" class="plan-list-item" :class="{ selected: plan.id === selectedId }" type="button" @click="load(plan.id)">
          <span class="plan-list-title">{{ plan.title }}</span><span class="badge">{{ plan.state }}</span>
        </button>
        <p v-if="plans.length === 0" class="panel-muted">{{ t(lang, "No plans yet.") }}</p>
      </aside>

      <article v-if="selected" class="plan-detail">
        <header class="plan-detail-head">
          <div><h2>{{ selected.title }}</h2><p><span class="badge accent">{{ selected.state }}</span> {{ done }}/{{ steps.length }} · {{ selected.agent }}<span v-if="selected.model"> · {{ selected.model }}</span></p></div>
          <div class="panel-actions"><button class="btn sm" type="button" @click="openEdit">{{ t(lang, "Edit") }}</button><button class="btn sm ghost danger" type="button" @click="deletePlan">{{ t(lang, "Delete") }}</button></div>
        </header>
        <p v-if="selected.goal" class="plan-goal">{{ selected.goal }}</p>
        <p v-if="selected.base" class="panel-muted">{{ t(lang, "Base branch:") }} {{ selected.base }}</p>

        <div class="plan-actions">
          <button v-if="canStart" class="btn primary" type="button" @click="action('Plan started', () => AppService.StartPlan(selected!.id))">{{ t(lang, "Start") }}</button>
          <button v-if="canPause" class="btn" type="button" @click="action('Plan paused', () => AppService.PausePlan(selected!.id))">{{ t(lang, "Pause") }}</button>
          <button v-if="canPause" class="btn" type="button" @click="action('Plan advanced', () => AppService.AdvancePlan(selected!.id))">{{ t(lang, "Advance") }}</button>
          <button v-if="canMerge" class="btn" type="button" @click="action('Plan merged', () => AppService.MergePlan(selected!.id))">{{ t(lang, "Merge") }}</button>
          <button v-if="canPR" class="btn" type="button" @click="action('Plan PR opened', () => AppService.OpenPlanPR(selected!.id))">{{ t(lang, "Open PR") }}</button>
        </div>
        <div class="plan-toggles">
          <label class="toggle-label"><input :checked="selected.auto" type="checkbox" @change="updatePlan({ auto: ($event.target as HTMLInputElement).checked })" />{{ t(lang, "Auto-advance") }}</label>
          <label class="toggle-label"><input :checked="selected.auto_approve" type="checkbox" @change="updatePlan({ auto_approve: ($event.target as HTMLInputElement).checked, auto: ($event.target as HTMLInputElement).checked || selected!.auto })" />{{ t(lang, "Auto-approve") }}</label>
        </div>

        <h3>{{ t(lang, "Steps") }}</h3>
        <div class="plan-steps">
          <div v-for="(step, index) in steps" :key="step.id" class="plan-step">
            <span class="step-number">{{ step.plan_step }}</span><button class="step-main" type="button" @click="openStep(step)"><strong>{{ step.title }}</strong><span>{{ step.status }} · {{ step.agent }}</span></button>
            <button class="btn sm ghost" type="button" :disabled="index === 0" :aria-label="t(lang, 'Move step up')" @click="moveStep(step, -1)">↑</button>
            <button class="btn sm ghost" type="button" :disabled="index === steps.length - 1" :aria-label="t(lang, 'Move step down')" @click="moveStep(step, 1)">↓</button>
            <button class="btn sm" type="button" @click="editingStep = step">{{ t(lang, "Edit") }}</button>
          </div>
        </div>
        <form class="add-step" @submit.prevent="addStep"><input v-model="stepTitle" :placeholder="t(lang, 'Step title')" :aria-label="t(lang, 'Step title')" /><button class="btn" type="submit" :disabled="!stepTitle.trim()">{{ t(lang, "Add step") }}</button></form>
      </article>
      <div v-else class="empty-state"><h2>{{ t(lang, "No plans yet.") }}</h2><p>{{ t(lang, "Create a plan to run its steps in order.") }}</p></div>
    </div>

    <div v-if="formOpen" class="modal-backdrop" role="dialog" aria-modal="true" :aria-label="t(lang, editing ? 'Edit plan' : 'New plan')" @click.self="formOpen = false">
      <div class="modal"><div class="modal-head"><h2>{{ t(lang, editing ? "Edit plan" : "New plan") }}</h2><button class="btn sm ghost" type="button" @click="formOpen = false">{{ t(lang, "Close") }}</button></div>
        <form class="form-grid" @submit.prevent="savePlan"><div class="field"><label for="plan-title">{{ t(lang, "Title") }}</label><input id="plan-title" v-model="title" required autofocus /></div><div class="field"><label for="plan-goal">{{ t(lang, "Goal") }}</label><textarea id="plan-goal" v-model="goal" rows="4" /></div><div class="form-row-2"><div class="field"><label for="plan-agent">{{ t(lang, "Agent") }}</label><select id="plan-agent" v-model="agent"><option v-for="item in AGENTS" :key="item" :value="item">{{ item }}</option></select></div><div class="field"><label for="plan-model">{{ t(lang, "Model") }}</label><input id="plan-model" v-model="model" :placeholder="t(lang, 'Optional')" /></div></div><div class="modal-actions"><button class="btn" type="button" @click="formOpen = false">{{ t(lang, "Cancel") }}</button><button class="btn primary" type="submit">{{ t(lang, "Save") }}</button></div></form>
      </div>
    </div>
    <TaskFormModal :open="!!editingStep" :project-id="project.id" :task="editingStep" :lang="lang" @close="editingStep = null" @saved="stepSaved" />
  </section>
</template>
