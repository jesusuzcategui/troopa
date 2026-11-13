<script setup lang="ts">
import { computed, ref, watch } from "vue";
import type { Profile } from "../../bindings/github.com/jesusuzcategui/troopa/internal/store/models";
import type { Settings } from "../../bindings/github.com/jesusuzcategui/troopa/cmd/troopa-desktop/models";
import * as AppService from "../../bindings/github.com/jesusuzcategui/troopa/internal/app/service";
import * as DesktopService from "../../bindings/github.com/jesusuzcategui/troopa/cmd/troopa-desktop/desktopservice";
import { t, type Lang } from "../i18n";

const props = defineProps<{ open: boolean; firstRun: boolean; settings: Settings | null; profiles: Profile[]; lang: Lang }>();
const emit = defineEmits<{ close: []; saved: [settings: Settings] }>();

const language = ref<Lang>("en");
const agent = ref("me");
const worktrees = ref("");
const terminal = ref("");
const account = ref("personal");
const method = ref<"key" | "login">("key");
const variable = ref("OPENAI_API_KEY");
const secret = ref("");
const profileName = ref("");
const saving = ref(false);
const error = ref("");
const notice = ref("");
const step = ref(0);

const usableAgents = computed(() => props.settings?.agents || []);
const selectedAgent = computed(() => usableAgents.value.find((item) => item.id === agent.value));

watch(() => [props.open, props.settings] as const, ([open, settings]) => {
  if (!open || !settings) return;
  language.value = settings.language === "es" ? "es" : "en";
  agent.value = settings.default_agent || (settings.agents || []).find((item) => item.installed)?.id || "me";
  worktrees.value = settings.worktrees || "";
  terminal.value = settings.terminal || "";
  step.value = 0;
  error.value = "";
  notice.value = "";
  secret.value = "";
}, { immediate: true });

function keyVariableForAgent() {
  if (agent.value === "claude") return "ANTHROPIC_API_KEY";
  if (agent.value === "codex") return "OPENAI_API_KEY";
  return "OPENAI_API_KEY";
}

watch(agent, () => { variable.value = keyVariableForAgent(); });

async function saveSettings(done: boolean) {
  await DesktopService.SaveSettings({ language: language.value, default_agent: agent.value, worktrees: worktrees.value.trim(), terminal: terminal.value.trim(), setup_done: done });
  const fresh = await DesktopService.GetSettings();
  emit("saved", fresh);
  return fresh;
}

async function continueSetup() {
  error.value = "";
  if (step.value === 0) {
    step.value = 1;
    return;
  }
  if (step.value === 1 && agent.value !== "me" && !selectedAgent.value?.installed) {
    error.value = t(props.lang, "Install this agent first, then scan again.");
    return;
  }
  if (step.value < 2) {
    step.value++;
    return;
  }
  await finish();
}

async function finish() {
  error.value = "";
  notice.value = "";
  saving.value = true;
  try {
    await saveSettings(true);
    if (agent.value !== "me") {
      if (!account.value.trim()) throw new Error(t(props.lang, "Give this account a name."));
      await DesktopService.CreateAccount(account.value.trim());
      if (method.value === "key") {
        if (!variable.value.trim() || !secret.value) throw new Error(t(props.lang, "Paste your API key to continue."));
        await DesktopService.SaveAccountKey(account.value.trim(), variable.value.trim(), secret.value);
      } else {
        await DesktopService.StartLogin(account.value.trim(), agent.value);
        notice.value = t(props.lang, "A terminal opened for sign-in. Finish there, then return here.");
      }
    }
    if (profileName.value.trim()) {
      await AppService.CreateProfile({ id: 0, name: profileName.value.trim(), profession: "", about: "", instructions: "", created_at: new Date().toISOString(), updated_at: new Date().toISOString() });
    }
    emit("close");
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    secret.value = "";
    saving.value = false;
  }
}

async function rescan() {
  const fresh = await DesktopService.GetSettings();
  emit("saved", fresh);
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" role="dialog" aria-modal="true" :aria-label="t(lang, firstRun ? 'Set up Troopa' : 'Settings')" @click.self="!firstRun && emit('close')">
    <section class="modal settings-modal">
      <div class="modal-head">
        <div><h2>{{ t(lang, firstRun ? "Set up Troopa" : "Settings") }}</h2><p>{{ t(lang, firstRun ? "A few choices, then you can create your first task." : "These choices apply to new tasks.") }}</p></div>
        <button v-if="!firstRun" class="btn sm ghost" type="button" @click="emit('close')">{{ t(lang, "Close") }}</button>
      </div>
      <div v-if="firstRun" class="setup-steps" aria-label="Setup progress"><span :class="{ active: step === 0 }">1</span><span :class="{ active: step === 1 }">2</span><span :class="{ active: step === 2 }">3</span></div>
      <div v-if="error" class="alert error" role="alert">{{ error }}</div><div v-if="notice" class="alert ok" role="status">{{ notice }}</div>

      <div v-show="!firstRun || step === 0" class="settings-section">
        <h3>{{ t(lang, "Language") }}</h3><p>{{ t(lang, "Choose the language you want to use every day.") }}</p>
        <select v-model="language"><option value="en">English</option><option value="es">Español</option></select>
      </div>
      <div v-show="!firstRun || step === 1" class="settings-section">
        <h3>{{ t(lang, "Choose an assistant") }}</h3><p>{{ t(lang, "Troopa checked what is already installed on this computer.") }}</p>
        <div class="agent-list"><label v-for="item in usableAgents" :key="item.id" class="agent-choice" :class="{ unavailable: !item.installed }"><input v-model="agent" type="radio" :value="item.id" /><span><strong>{{ item.name }}</strong><small v-if="item.installed">{{ t(lang, "Ready to use") }}{{ item.version ? ` · ${item.version}` : "" }}</small><small v-else>{{ t(lang, "Not installed. Install it from its website, then scan again.") }}</small></span></label><label class="agent-choice"><input v-model="agent" type="radio" value="me" /><span><strong>{{ t(lang, "I will do the work myself") }}</strong><small>{{ t(lang, "You can add an assistant later in Settings.") }}</small></span></label></div>
        <button class="btn sm" type="button" @click="rescan">{{ t(lang, "Scan again") }}</button>
      </div>
      <div v-show="!firstRun || step === 2" class="settings-section">
        <h3>{{ t(lang, "Where work is kept") }}</h3><p>{{ t(lang, "Troopa makes a separate safe copy of each project here while an assistant works.") }}</p>
        <label class="field"><span>{{ t(lang, "Work folder") }}</span><input v-model="worktrees" :placeholder="t(lang, 'Leave empty to use Troopa’s folder')" /></label>
        <label class="field"><span>{{ t(lang, "Terminal for working together") }}</span><input v-model="terminal" :placeholder="t(lang, 'For example: kitty -e')" /></label>
      </div>
      <div v-show="!firstRun || step === 2" class="settings-section">
        <h3>{{ t(lang, "Account") }}</h3><p>{{ t(lang, "Use a sign-in or an API key. Your key is stored in your computer’s secure keyring, not in this app.") }}</p>
        <template v-if="agent !== 'me'"><label class="field"><span>{{ t(lang, "Account name") }}</span><input v-model="account" /></label><div class="method-row"><label><input v-model="method" type="radio" value="key" /> {{ t(lang, "Paste an API key") }}</label><label><input v-model="method" type="radio" value="login" /> {{ t(lang, "Sign in in a terminal") }}</label></div><template v-if="method === 'key'"><label class="field"><span>{{ t(lang, "Key name") }}</span><input v-model="variable" /></label><label class="field"><span>{{ t(lang, "API key") }}</span><input v-model="secret" type="password" autocomplete="off" /></label></template></template><p v-else>{{ t(lang, "Choose an assistant whenever you are ready.") }}</p>
      </div>
      <div v-show="!firstRun" class="settings-section"><h3>{{ t(lang, "Profile") }}</h3><p>{{ t(lang, "Tell assistants who they work for. This is optional.") }}</p><label class="field"><span>{{ t(lang, "New profile name") }}</span><input v-model="profileName" :placeholder="t(lang, 'For example: Ana, a bakery owner')" /></label><p v-if="profiles.length">{{ profiles.map((profile) => profile.name).join(", ") }}</p></div>
      <div class="modal-actions"><button v-if="firstRun && step > 0" class="btn" type="button" :disabled="saving" @click="step--">{{ t(lang, "Back") }}</button><button v-if="!firstRun" class="btn" type="button" :disabled="saving" @click="emit('close')">{{ t(lang, "Cancel") }}</button><button class="btn primary" type="button" :disabled="saving" @click="firstRun ? continueSetup() : finish()">{{ saving ? t(lang, "Saving…") : firstRun && step < 2 ? t(lang, "Continue") : t(lang, "Save and continue") }}</button></div>
    </section>
  </div>
</template>
