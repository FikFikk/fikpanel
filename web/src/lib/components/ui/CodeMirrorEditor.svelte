<script lang="ts">
  import { onMount } from 'svelte';
  import { EditorState, Compartment, type Extension } from '@codemirror/state';
  import {
    EditorView,
    keymap,
    lineNumbers,
    highlightActiveLineGutter,
    highlightSpecialChars,
    drawSelection,
    dropCursor,
    rectangularSelection,
    crosshairCursor,
    highlightActiveLine,
    placeholder as cmPlaceholder,
  } from '@codemirror/view';
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
  import {
    HighlightStyle,
    StreamLanguage,
    bracketMatching,
    foldGutter,
    foldKeymap,
    indentOnInput,
    syntaxHighlighting,
  } from '@codemirror/language';
  import { searchKeymap, highlightSelectionMatches, openSearchPanel } from '@codemirror/search';
  import { autocompletion, completionKeymap, closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete';
  import { tags as t } from '@lezer/highlight';

  import { javascript } from '@codemirror/lang-javascript';
  import { json } from '@codemirror/lang-json';
  import { yaml } from '@codemirror/lang-yaml';
  import { html } from '@codemirror/lang-html';
  import { css } from '@codemirror/lang-css';
  import { php } from '@codemirror/lang-php';
  import { python } from '@codemirror/lang-python';
  import { sql } from '@codemirror/lang-sql';
  import { markdown } from '@codemirror/lang-markdown';

  import { nginx } from '@codemirror/legacy-modes/mode/nginx';
  import { shell } from '@codemirror/legacy-modes/mode/shell';
  import { dockerFile } from '@codemirror/legacy-modes/mode/dockerfile';
  import { properties } from '@codemirror/legacy-modes/mode/properties';
  import { go } from '@codemirror/legacy-modes/mode/go';
  import { toml } from '@codemirror/legacy-modes/mode/toml';

  export type EditorLang =
    | 'auto'
    | 'nginx'
    | 'yaml'
    | 'json'
    | 'php'
    | 'javascript'
    | 'typescript'
    | 'html'
    | 'css'
    | 'sql'
    | 'python'
    | 'markdown'
    | 'shell'
    | 'dockerfile'
    | 'ini'
    | 'go'
    | 'toml'
    | 'plain';

  interface Props {
    value: string;
    lang?: EditorLang;
    filename?: string;
    readonly?: boolean;
    wordWrap?: boolean;
    fontSize?: number;
    minHeight?: string;
    maxHeight?: string;
    fill?: boolean;
    placeholder?: string;
    onSave?: () => void;
    onCursorChange?: (pos: { line: number; col: number }) => void;
  }

  let {
    value = $bindable(''),
    lang = 'auto',
    filename = '',
    readonly = false,
    wordWrap = false,
    fontSize = 13,
    minHeight = '260px',
    maxHeight = '',
    fill = false,
    placeholder = '',
    onSave,
    onCursorChange,
  }: Props = $props();

  let hostEl = $state<HTMLDivElement | null>(null);
  let view: EditorView | null = null;
  let internalSync = false;

  const langComp = new Compartment();
  const wrapComp = new Compartment();
  const readOnlyComp = new Compartment();

  // Skema warna sintaks berbasis CSS design tokens agar otomatis adaptif pada mode Gelap, Terang, dan seluruh palet aksen
  const tokenHighlightStyle = HighlightStyle.define([
    { tag: [t.keyword, t.modifier, t.operatorKeyword, t.controlKeyword], color: 'var(--accent-strong)', fontWeight: '600' },
    { tag: [t.string, t.special(t.string), t.regexp], color: 'var(--success-strong)' },
    { tag: [t.number, t.bool, t.null, t.atom], color: 'var(--warning-strong)' },
    { tag: [t.comment, t.lineComment, t.blockComment, t.docComment], color: 'var(--text-subtle)', fontStyle: 'italic' },
    { tag: [t.variableName, t.propertyName, t.attributeName], color: 'var(--info)' },
    { tag: [t.function(t.variableName), t.function(t.propertyName), t.definition(t.variableName)], color: 'var(--violet)' },
    { tag: [t.typeName, t.className, t.tagName, t.namespace], color: 'var(--warning-strong)' },
    { tag: [t.operator, t.punctuation, t.separator], color: 'var(--text-muted)' },
    { tag: [t.heading, t.strong], color: 'var(--text)', fontWeight: '700' },
    { tag: [t.emphasis], fontStyle: 'italic' },
    { tag: [t.link, t.url], color: 'var(--info)', textDecoration: 'underline' },
    { tag: [t.invalid], color: 'var(--danger-strong)', textDecoration: 'underline' },
  ]);

  // Deteksi bahasa syntax highlighting otomatis dari prop lang eksplisit atau nama berkas
  function resolveLanguageExtension(explicitLang: EditorLang, name: string): Extension {
    let target: EditorLang = explicitLang;
    if (target === 'auto') {
      const lower = (name || '').toLowerCase();
      const base = lower.split(/[\\/]/).pop() || '';
      const ext = base.includes('.') ? base.split('.').pop() || '' : '';

      if (base === 'dockerfile' || ext === 'dockerfile') target = 'dockerfile';
      else if (base === 'nginx.conf' || lower.includes('nginx') || lower.includes('sites-available') || lower.includes('sites-enabled')) target = 'nginx';
      else if (base.startsWith('.env') || ext === 'env' || ext === 'ini' || ext === 'cfg' || ext === 'cnf' || ext === 'properties' || ext === 'service' || ext === 'timer') target = 'ini';
      else if (ext === 'conf') target = 'nginx';
      else if (ext === 'yml' || ext === 'yaml') target = 'yaml';
      else if (ext === 'json' || ext === 'jsonc') target = 'json';
      else if (ext === 'php' || ext === 'phtml') target = 'php';
      else if (ext === 'ts' || ext === 'tsx') target = 'typescript';
      else if (ext === 'js' || ext === 'mjs' || ext === 'cjs' || ext === 'jsx') target = 'javascript';
      else if (ext === 'html' || ext === 'htm' || ext === 'xml' || ext === 'svg' || ext === 'svelte' || ext === 'vue') target = 'html';
      else if (ext === 'css' || ext === 'scss' || ext === 'less') target = 'css';
      else if (ext === 'sql') target = 'sql';
      else if (ext === 'py') target = 'python';
      else if (ext === 'md' || ext === 'markdown') target = 'markdown';
      else if (ext === 'sh' || ext === 'bash' || ext === 'zsh' || base === '.bashrc' || base === '.profile') target = 'shell';
      else if (ext === 'go') target = 'go';
      else if (ext === 'toml') target = 'toml';
      else target = 'plain';
    }

    switch (target) {
      case 'nginx':
        return StreamLanguage.define(nginx);
      case 'yaml':
        return yaml();
      case 'json':
        return json();
      case 'php':
        return php();
      case 'typescript':
        return javascript({ typescript: true, jsx: filename.toLowerCase().endsWith('.tsx') });
      case 'javascript':
        return javascript({ jsx: filename.toLowerCase().endsWith('.jsx') });
      case 'html':
        return html();
      case 'css':
        return css();
      case 'sql':
        return sql();
      case 'python':
        return python();
      case 'markdown':
        return markdown();
      case 'shell':
        return StreamLanguage.define(shell);
      case 'dockerfile':
        return StreamLanguage.define(dockerFile);
      case 'ini':
        return StreamLanguage.define(properties);
      case 'go':
        return StreamLanguage.define(go);
      case 'toml':
        return StreamLanguage.define(toml);
      default:
        return [];
    }
  }

  function emitCursor(state: EditorState) {
    if (!onCursorChange) return;
    const head = state.selection.main.head;
    const lineObj = state.doc.lineAt(head);
    onCursorChange({
      line: lineObj.number,
      col: head - lineObj.from + 1,
    });
  }

  onMount(() => {
    if (!hostEl) return;

    const extensions: Extension[] = [
      lineNumbers(),
      highlightActiveLineGutter(),
      highlightSpecialChars(),
      history(),
      foldGutter(),
      drawSelection(),
      dropCursor(),
      EditorState.allowMultipleSelections.of(true),
      indentOnInput(),
      syntaxHighlighting(tokenHighlightStyle, { fallback: true }),
      bracketMatching(),
      closeBrackets(),
      autocompletion(),
      rectangularSelection(),
      crosshairCursor(),
      highlightActiveLine(),
      highlightSelectionMatches(),
      keymap.of([
        ...closeBracketsKeymap,
        ...defaultKeymap,
        ...searchKeymap,
        ...historyKeymap,
        ...foldKeymap,
        ...completionKeymap,
        indentWithTab,
        {
          key: 'Mod-s',
          preventDefault: true,
          run: () => {
            onSave?.();
            return true;
          },
        },
      ]),
      langComp.of(resolveLanguageExtension(lang, filename)),
      wrapComp.of(wordWrap ? EditorView.lineWrapping : []),
      readOnlyComp.of(EditorState.readOnly.of(readonly)),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          internalSync = true;
          value = update.state.doc.toString();
          internalSync = false;
        }
        if (update.selectionSet || update.docChanged) {
          emitCursor(update.state);
        }
      }),
    ];

    if (placeholder) {
      extensions.push(cmPlaceholder(placeholder));
    }

    const state = EditorState.create({
      doc: value ?? '',
      extensions,
    });

    view = new EditorView({
      state,
      parent: hostEl,
    });

    emitCursor(view.state);

    return () => {
      view?.destroy();
      view = null;
    };
  });

  // Sinkronkan perubahan value dari luar (misal muat berkas baru, reset, atau ganti preset) tanpa mengganggu undo/kursor saat mengetik
  $effect(() => {
    const nextDoc = value ?? '';
    if (!view || internalSync) return;
    const curDoc = view.state.doc.toString();
    if (nextDoc !== curDoc) {
      view.dispatch({
        changes: { from: 0, to: curDoc.length, insert: nextDoc },
      });
    }
  });

  // Rekonfigurasi bahasa secara dinamis bila filename atau prop lang berubah
  $effect(() => {
    const ext = resolveLanguageExtension(lang, filename);
    if (!view) return;
    view.dispatch({ effects: langComp.reconfigure(ext) });
  });

  // Rekonfigurasi word-wrap secara dinamis
  $effect(() => {
    const wrapped = wordWrap;
    if (!view) return;
    view.dispatch({ effects: wrapComp.reconfigure(wrapped ? EditorView.lineWrapping : []) });
  });

  // Rekonfigurasi status read-only secara dinamis
  $effect(() => {
    const ro = readonly;
    if (!view) return;
    view.dispatch({ effects: readOnlyComp.reconfigure(EditorState.readOnly.of(ro)) });
  });

  // ── API publik komponen untuk kontrol toolbar eksternal ──
  export function openSearch() {
    if (!view) return;
    openSearchPanel(view);
    view.focus();
  }

  export function gotoLine(lineNumber: number) {
    if (!view) return;
    const clamped = Math.max(1, Math.min(lineNumber, view.state.doc.lines));
    const lineInfo = view.state.doc.line(clamped);
    view.dispatch({
      selection: { anchor: lineInfo.from },
      effects: EditorView.scrollIntoView(lineInfo.from, { y: 'center' }),
    });
    view.focus();
  }

  export function focus() {
    view?.focus();
  }

  let wrapperStyle = $derived(
    `--cm-font-size: ${fontSize}px; --cm-min-h: ${minHeight};${maxHeight ? ` --cm-max-h: ${maxHeight};` : ''}`,
  );
</script>

<div class="cm-host" class:cm-host--fill={fill} style={wrapperStyle} bind:this={hostEl}></div>
