<script setup lang="ts">
import { t, type Lang } from "../i18n";

defineProps<{
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
  lang: Lang;
}>();

const emit = defineEmits<{
  close: [];
  confirm: [];
}>();
</script>

<template>
  <div
    v-if="open"
    class="modal-backdrop"
    role="dialog"
    aria-modal="true"
    :aria-label="title"
    @click.self="emit('close')"
    @keydown.esc="emit('close')"
  >
    <div class="modal confirm-modal">
      <div class="modal-head">
        <h2>{{ title }}</h2>
        <button class="btn sm ghost" type="button" @click="emit('close')">
          {{ t(lang, "Close") }}
        </button>
      </div>
      <div class="modal-body">
        <p class="confirm-message">{{ message }}</p>
      </div>
      <div class="modal-actions">
        <button class="btn" type="button" @click="emit('close')">
          {{ t(lang, "Cancel") }}
        </button>
        <button
          class="btn primary"
          :class="{ danger: danger }"
          type="button"
          @click="emit('confirm')"
        >
          {{ confirmLabel || t(lang, "Confirm") }}
        </button>
      </div>
    </div>
  </div>
</template>
