<script setup lang="ts">
import { ref, watch } from "vue";
import type { Client } from "../../bindings/github.com/jesusuzcategui/troopa/internal/store/models";
import * as AppService from "../../bindings/github.com/jesusuzcategui/troopa/internal/app/service";
import { t, type Lang } from "../i18n";

const props = defineProps<{ open: boolean; clients: Client[]; lang: Lang }>();
const emit = defineEmits<{ close: []; changed: [client: Client, isNew: boolean] }>();

const editing = ref<Client | null>(null);
const name = ref("");
const contact = ref("");
const notes = ref("");
const saving = ref(false);
const error = ref("");

watch(
  () => props.open,
  (open) => {
    if (open) reset();
  },
);

function reset(client: Client | null = null) {
  editing.value = client;
  name.value = client?.name ?? "";
  contact.value = client?.contact ?? "";
  notes.value = client?.notes ?? "";
  error.value = "";
}

async function save() {
  error.value = "";
  if (!name.value.trim()) {
    error.value = t(props.lang, "Name is required");
    return;
  }
  saving.value = true;
  try {
    if (editing.value) {
      const updated = { ...editing.value, name: name.value, contact: contact.value, notes: notes.value };
      await AppService.UpdateClient(updated);
      emit("changed", updated, false);
    } else {
      const created = await AppService.CreateClient({
        id: 0,
        name: name.value,
        contact: contact.value,
        notes: notes.value,
        archived: false,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      });
      emit("changed", created, true);
    }
    reset();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  } finally {
    saving.value = false;
  }
}

async function archive(client: Client) {
  error.value = "";
  try {
    await AppService.ArchiveClient(client.id);
    emit("changed", { ...client, archived: true }, false);
    if (editing.value?.id === client.id) reset();
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err);
  }
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" role="dialog" aria-modal="true" :aria-label="t(lang, 'Clients')" @click.self="emit('close')" @keydown.esc="emit('close')">
    <div class="modal clients-modal">
      <div class="modal-head">
        <h2>{{ t(lang, "Clients") }}</h2>
        <button class="btn sm ghost" type="button" @click="emit('close')">{{ t(lang, "Close") }}</button>
      </div>
      <div v-if="error" class="alert error" role="alert">{{ error }}</div>
      <div class="clients-layout">
        <div class="clients-list">
          <button class="client-list-item" :class="{ selected: !editing }" type="button" @click="reset()">+ {{ t(lang, "New client") }}</button>
          <div v-for="client in clients" :key="client.id" class="client-list-item" :class="{ selected: editing?.id === client.id, archived: client.archived }">
            <button type="button" @click="reset(client)">{{ client.name }}</button>
            <button v-if="!client.archived" class="btn sm ghost danger" type="button" @click="archive(client)">{{ t(lang, "Archive") }}</button>
          </div>
        </div>
        <form class="form-grid" @submit.prevent="save">
          <h3>{{ editing ? t(lang, "Edit client") : t(lang, "New client") }}</h3>
          <div class="field"><label for="client-name">{{ t(lang, "Name") }}</label><input id="client-name" v-model="name" required autofocus /></div>
          <div class="field"><label for="client-contact">{{ t(lang, "Contact") }}</label><input id="client-contact" v-model="contact" /></div>
          <div class="field"><label for="client-notes">{{ t(lang, "Notes") }}</label><textarea id="client-notes" v-model="notes"></textarea></div>
          <div class="modal-actions"><button class="btn primary" :disabled="saving" type="submit">{{ saving ? t(lang, "Saving…") : editing ? t(lang, "Save") : t(lang, "Create") }}</button></div>
        </form>
      </div>
    </div>
  </div>
</template>
