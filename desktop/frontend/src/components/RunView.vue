<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import type { ReviewComment, Round, Run } from "../../bindings/github.com/jesusuzcategui/troopa/internal/store/models";
import * as AppService from "../../bindings/github.com/jesusuzcategui/troopa/internal/app/service";
import { SYNC_MS } from "../constants";
import { t, type Lang } from "../i18n";

type DiffLine = { kind: string; text: string; old: number; new: number };
type FileDiff = { path: string; lines: DiffLine[] };

const props = defineProps<{ runId: number; taskTitle: string; lang: Lang; embedded?: boolean }>();
const emit = defineEmits<{ back: []; notice: [message: string]; error: [message: string] }>();

const run = ref<Run | null>(null);
const rounds = ref<Round[]>([]);
const comments = ref<ReviewComment[]>([]);
const files = ref<FileDiff[]>([]);
const log = ref("");
const tab = ref<"summary" | "changes" | "log">("summary");
const loading = ref(true);
const feedback = ref("");
const commentFor = ref<{ path: string; line: DiffLine } | null>(null);
const commentBody = ref("");
let timer: ReturnType<typeof setInterval> | null = null;

const final = computed(() => ["merged", "cleaned", "discarded"].includes(run.value?.state ?? ""));
const canRequestChanges = computed(() => ["review", "blocked", "approved"].includes(run.value?.state ?? ""));
const canApprove = computed(() => run.value?.state === "review");
const canFinish = computed(() => run.value?.state === "approved");

function lineNumber(line: DiffLine): number {
  return line.kind === "-" ? line.old : line.new;
}

function lineComments(path: string, line: DiffLine): ReviewComment[] {
  const side = line.kind === "-" ? "-" : line.kind === "+" ? "+" : " ";
  return comments.value.filter((comment) => comment.path === path && comment.line === lineNumber(line) && comment.side === side);
}

async function load(withDiff = false) {
  try {
    const current = await AppService.GetRun(props.runId);
    run.value = current;
    const [nextRounds, nextComments, nextLog] = await Promise.all([
      AppService.ListRunRounds(props.runId),
      AppService.ListRunComments(props.runId),
      AppService.RunLog(props.runId),
    ]);
    rounds.value = nextRounds ?? [];
    comments.value = nextComments ?? [];
    log.value = nextLog;
    if (withDiff || tab.value === "changes") {
      files.value = ((await AppService.RunDiff(props.runId)) ?? []).map((file) => ({
        path: file.path,
        lines: file.lines ?? [],
      }));
    }
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  } finally {
    loading.value = false;
  }
}

async function selectTab(next: "summary" | "changes" | "log") {
  tab.value = next;
  if (next === "changes") await load(true);
}

async function action(label: string, call: () => Promise<void>) {
  try {
    await call();
    emit("notice", label);
    await load(true);
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

async function requestChanges() {
  if (!feedback.value.trim()) return;
  await action(t(props.lang, "Changes requested"), () => AppService.RequestRunChanges(props.runId, feedback.value));
  feedback.value = "";
}

function beginComment(path: string, line: DiffLine) {
  if (final.value || lineNumber(line) === 0) return;
  commentFor.value = { path, line };
  commentBody.value = "";
}

async function saveComment() {
  const target = commentFor.value;
  if (!target || !commentBody.value.trim() || !run.value) return;
  const line = target.line;
  try {
    await AppService.CreateComment({
      id: 0,
      run_id: run.value.id,
      round: rounds.value.at(-1)?.number ?? 0,
      path: target.path,
      line: lineNumber(line),
      side: line.kind === "-" ? "-" : line.kind === "+" ? "+" : " ",
      quote: line.text.replace(/^[+\- ]/, ""),
      body: commentBody.value.trim(),
      resolved: false,
      created_at: "",
    });
    commentFor.value = null;
    await load();
  } catch (err) {
    emit("error", err instanceof Error ? err.message : String(err));
  }
}

onMounted(async () => {
  await load(true);
  timer = setInterval(() => { void load(); }, SYNC_MS);
});
onUnmounted(() => { if (timer) clearInterval(timer); });
</script>

<template>
  <section class="run-view" :aria-label="t(lang, 'Run')">
    <header v-if="!embedded" class="run-head">
      <div>
        <button class="btn sm ghost" type="button" @click="emit('back')">← {{ t(lang, 'Task board') }}</button>
        <h1 class="page-title">{{ taskTitle }}</h1>
        <p class="page-subtitle">
          <span class="badge accent">{{ run ? t(lang, run.state) : t(lang, 'Loading run…') }}</span>
          <span v-if="run?.branch">{{ run.branch }}</span>
        </p>
      </div>
      <div class="run-actions">
        <button v-if="canApprove" class="btn primary" type="button" @click="action(t(lang, 'Run approved'), () => AppService.ApproveRun(runId))">{{ t(lang, 'Approve') }}</button>
        <button v-if="canFinish" class="btn" type="button" @click="action(t(lang, 'Merged'), () => AppService.MergeRun(runId))">{{ t(lang, 'Merge') }}</button>
        <button v-if="canFinish" class="btn" type="button" @click="action(t(lang, 'PR opened'), () => AppService.OpenRunPR(runId))">{{ t(lang, 'Open PR') }}</button>
        <button v-if="canFinish" class="btn" type="button" @click="action(t(lang, 'Worktree cleaned'), () => AppService.CleanRun(runId))">{{ t(lang, 'Clean') }}</button>
        <button v-if="!final" class="btn ghost danger" type="button" @click="action(t(lang, 'Run discarded'), () => AppService.DiscardRun(runId))">{{ t(lang, 'Discard') }}</button>
        <button class="btn" type="button" @click="action(t(lang, 'Editor opened'), () => AppService.OpenRunEditor(runId))">{{ t(lang, 'Open in editor') }}</button>
      </div>
    </header>

    <div class="run-tabs" role="tablist" :aria-label="t(lang, 'Run tabs')">
      <button :class="{ active: tab === 'summary' }" type="button" role="tab" :aria-selected="tab === 'summary'" @click="selectTab('summary')">{{ t(lang, 'Summary') }}</button>
      <button :class="{ active: tab === 'changes' }" type="button" role="tab" :aria-selected="tab === 'changes'" @click="selectTab('changes')">{{ t(lang, 'Changes') }} <span v-if="files.length">{{ files.length }}</span></button>
      <button :class="{ active: tab === 'log' }" type="button" role="tab" :aria-selected="tab === 'log'" @click="selectTab('log')">{{ t(lang, 'Log') }}</button>
    </div>

    <div v-if="loading" class="empty-state"><p>{{ t(lang, 'Loading run…') }}</p></div>
    <div v-else-if="tab === 'summary'" class="run-pane">
      <article v-for="round in rounds" :key="round.id || round.number" class="round-item">
        <header><strong>{{ t(lang, 'Round') }} {{ round.number }}</strong><span class="badge">{{ t(lang, round.state) }}</span></header>
        <p v-if="round.feedback" class="round-feedback">{{ round.feedback }}</p>
        <pre v-if="round.summary" class="run-copy">{{ round.summary }}</pre>
        <p v-else class="panel-muted">{{ t(lang, 'Waiting for the agent response…') }}</p>
      </article>
    </div>
    <div v-else-if="tab === 'changes'" class="run-pane diff-pane">
      <p v-if="files.length === 0" class="panel-muted">{{ t(lang, 'No changes yet.') }}</p>
      <article v-for="file in files" :key="file.path" class="diff-file">
        <h2>{{ file.path }}</h2>
        <div v-for="(line, index) in file.lines" :key="index" class="diff-row" :class="`diff-${line.kind || 'plain'}`">
          <button class="diff-code" type="button" :disabled="final || lineNumber(line) === 0" @click="beginComment(file.path, line)">
            <span class="diff-number">{{ lineNumber(line) || '' }}</span><code>{{ line.text }}</code>
          </button>
          <div v-for="comment in lineComments(file.path, line)" :key="comment.id" class="line-comment">{{ comment.body }}</div>
        </div>
      </article>
      <form v-if="commentFor" class="comment-box" @submit.prevent="saveComment">
        <label>{{ t(lang, 'Comment on %s:%d').replace('%s', commentFor.path).replace('%d', String(lineNumber(commentFor.line))) }}</label>
        <textarea v-model="commentBody" :placeholder="t(lang, 'Comment on this line')" autofocus />
        <div><button class="btn sm" type="button" @click="commentFor = null">{{ t(lang, 'Cancel') }}</button><button class="btn sm primary" type="submit">{{ t(lang, 'Save comment') }}</button></div>
      </form>
    </div>
    <div v-else class="run-pane"><pre class="run-log">{{ log || t(lang, 'No log yet.') }}</pre></div>

    <form v-if="canRequestChanges" class="changes-form" @submit.prevent="requestChanges">
      <label for="run-feedback">{{ t(lang, 'Request changes') }}</label>
      <textarea id="run-feedback" v-model="feedback" :placeholder="t(lang, 'What should change?')" />
      <button class="btn" type="submit" :disabled="!feedback.trim()">{{ t(lang, 'Send changes') }}</button>
    </form>
  </section>
</template>
