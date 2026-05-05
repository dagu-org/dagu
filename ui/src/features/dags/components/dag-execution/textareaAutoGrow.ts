// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

const maxTextareaHeight = 150;

export function autoGrowTextarea(el: HTMLTextAreaElement) {
  el.style.height = 'auto';
  const clamped = Math.min(el.scrollHeight, maxTextareaHeight);
  el.style.height = `${clamped}px`;
  el.style.overflowY = el.scrollHeight > maxTextareaHeight ? 'auto' : 'hidden';
}
