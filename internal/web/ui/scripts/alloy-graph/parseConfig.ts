/**
 * parseConfig.ts — static analysis of an Alloy configuration file.
 *
 * Produces a list of GraphableComponent objects (compatible with layoutGraph)
 * without requiring a running Alloy instance.
 *
 * The parser is intentionally minimal: it handles block declarations, nested
 * bodies, quoted strings, line/block comments, and dotted-identifier traversals.
 * It does not evaluate expressions.
 *
 * Edge direction heuristic (mirrors loader.go `setDataFlowEdges`):
 *   - Component A references B.<pushField>  →  data flows A → B  (push)
 *   - Component A references B.<other>      →  data flows B → A  (pull/provide)
 *
 * Push fields: receiver, input, handler, append.
 */

import type { GraphableComponent } from '../../src/features/graph/layoutGraph.ts';

// ── Tokeniser ────────────────────────────────────────────────────────────────

type Token =
  | { kind: 'ident'; value: string }
  | { kind: 'string'; value: string }
  | { kind: 'lbrace' }
  | { kind: 'rbrace' }
  | { kind: 'lbracket' }
  | { kind: 'rbracket' }
  | { kind: 'lparen' }
  | { kind: 'rparen' }
  | { kind: 'dot' }
  | { kind: 'comma' }
  | { kind: 'eq' }
  | { kind: 'other'; value: string };

function tokenise(src: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;

  while (i < src.length) {
    // Line comment.
    if (src[i] === '/' && src[i + 1] === '/') {
      while (i < src.length && src[i] !== '\n') i++;
      continue;
    }
    // Block comment.
    if (src[i] === '/' && src[i + 1] === '*') {
      i += 2;
      while (i < src.length && !(src[i] === '*' && src[i + 1] === '/')) i++;
      i += 2;
      continue;
    }
    // Whitespace.
    if (/\s/.test(src[i])) {
      i++;
      continue;
    }
    // Double-quoted string (handle escapes minimally).
    if (src[i] === '"') {
      let s = '';
      i++; // skip opening quote
      while (i < src.length && src[i] !== '"') {
        if (src[i] === '\\') {
          i++; // skip backslash
          s += src[i] ?? '';
        } else {
          s += src[i];
        }
        i++;
      }
      i++; // skip closing quote
      tokens.push({ kind: 'string', value: s });
      continue;
    }
    // Backtick raw string.
    if (src[i] === '`') {
      let s = '';
      i++;
      while (i < src.length && src[i] !== '`') {
        s += src[i++];
      }
      i++;
      tokens.push({ kind: 'string', value: s });
      continue;
    }
    // Identifier or keyword.
    if (/[a-zA-Z_]/.test(src[i])) {
      let s = '';
      while (i < src.length && /[\w]/.test(src[i])) s += src[i++];
      tokens.push({ kind: 'ident', value: s });
      continue;
    }
    // Single-character tokens.
    switch (src[i]) {
      case '{':
        tokens.push({ kind: 'lbrace' });
        break;
      case '}':
        tokens.push({ kind: 'rbrace' });
        break;
      case '[':
        tokens.push({ kind: 'lbracket' });
        break;
      case ']':
        tokens.push({ kind: 'rbracket' });
        break;
      case '(':
        tokens.push({ kind: 'lparen' });
        break;
      case ')':
        tokens.push({ kind: 'rparen' });
        break;
      case '.':
        tokens.push({ kind: 'dot' });
        break;
      case ',':
        tokens.push({ kind: 'comma' });
        break;
      case '=':
        tokens.push({ kind: 'eq' });
        break;
      default:
        tokens.push({ kind: 'other', value: src[i] });
    }
    i++;
  }

  return tokens;
}

// ── Block tree ────────────────────────────────────────────────────────────────

interface Block {
  /** E.g. "prometheus.scrape" */
  name: string;
  /** Instance label, e.g. "web". Empty string for singletons. */
  label: string;
  /** Tokens that make up the body of this block (between { and }). */
  bodyTokens: Token[];
  /** depth at which this block was declared (0 = top-level) */
  depth: number;
}

/**
 * extractBlocks performs a single-pass scan over the token stream, collecting
 * every block declaration at every nesting depth.
 */
function extractBlocks(tokens: Token[]): Block[] {
  const blocks: Block[] = [];
  let i = 0;

  // Stack of open blocks: each entry holds the block + the token index where
  // its body started (right after the opening {).
  const stack: Array<{ block: Block; bodyStart: number }> = [];

  while (i < tokens.length) {
    const t = tokens[i];

    // Detect a block header: ident (dot ident)* [string] {
    // We need to look ahead without consuming, so collect a candidate name.
    if (t.kind === 'ident') {
      let j = i;
      const nameParts: string[] = [t.value];

      // Consume dotted name segments.
      while (
        j + 2 < tokens.length &&
        tokens[j + 1].kind === 'dot' &&
        tokens[j + 2].kind === 'ident'
      ) {
        nameParts.push((tokens[j + 2] as { kind: 'ident'; value: string }).value);
        j += 2;
      }
      j++;

      const name = nameParts.join('.');
      let label = '';

      // Optional string label.
      if (j < tokens.length && tokens[j].kind === 'string') {
        label = (tokens[j] as { kind: 'string'; value: string }).value;
        j++;
      }

      // Must be followed by { to be a block.
      if (j < tokens.length && tokens[j].kind === 'lbrace') {
        // Only treat multi-segment names as components (e.g. "prometheus.scrape").
        // Single-segment blocks like "logging" or nested utility blocks like
        // "endpoint" are still recorded so we can walk their bodies.
        const block: Block = {
          name,
          label,
          bodyTokens: [],
          depth: stack.length,
        };
        stack.push({ block, bodyStart: j + 1 });
        blocks.push(block);
        i = j + 1; // position after opening brace
        continue;
      }
    }

    // Opening brace not part of a detected block header — just track depth.
    if (t.kind === 'lbrace') {
      // Synthetic block to keep depth accounting consistent.
      const block: Block = { name: '', label: '', bodyTokens: [], depth: stack.length };
      stack.push({ block, bodyStart: i + 1 });
      i++;
      continue;
    }

    if (t.kind === 'rbrace') {
      const entry = stack.pop();
      if (entry) {
        entry.block.bodyTokens = tokens.slice(entry.bodyStart, i);
      }
      i++;
      continue;
    }

    i++;
  }

  return blocks;
}

// ── Traversal extraction ──────────────────────────────────────────────────────

/**
 * findTraversals scans a token slice for dotted-identifier chains.
 * Returns an array of traversal strings like ["prometheus", "remote_write", "default", "receiver"].
 */
function findTraversals(bodyTokens: Token[]): string[][] {
  const traversals: string[][] = [];
  let i = 0;

  while (i < bodyTokens.length) {
    if (bodyTokens[i].kind === 'ident') {
      // Try to build a dotted chain.
      const chain: string[] = [(bodyTokens[i] as { kind: 'ident'; value: string }).value];
      let j = i + 1;
      while (
        j + 1 < bodyTokens.length &&
        bodyTokens[j].kind === 'dot' &&
        bodyTokens[j + 1].kind === 'ident'
      ) {
        chain.push((bodyTokens[j + 1] as { kind: 'ident'; value: string }).value);
        j += 2;
      }
      if (chain.length >= 2) {
        traversals.push(chain);
      }
      i = j;
      continue;
    }
    i++;
  }

  return traversals;
}

// ── Edge direction heuristic ──────────────────────────────────────────────────

/**
 * Fields that indicate a "push" relationship:
 * when A references B.<pushField>, data flows A → B.
 *
 * All other field names indicate a "pull/provide" relationship:
 * data flows B → A (reversed).
 */
const PUSH_FIELDS = new Set(['receiver', 'input', 'handler', 'append']);

// ── Public API ────────────────────────────────────────────────────────────────

/**
 * parseConfig statically analyses raw Alloy config source and returns a list of
 * GraphableComponent objects ready for `layoutGraph`.
 */
export function parseConfig(source: string): GraphableComponent[] {
  const tokens = tokenise(source);
  const blocks = extractBlocks(tokens);

  // Index only top-level component blocks (depth 0, multi-segment name).
  const componentIdx = new Map<string, number>(); // component id → index
  const components: GraphableComponent[] = [];

  for (const block of blocks) {
    if (block.depth !== 0 || block.name.split('.').length < 2) continue;

    const id = block.label ? `${block.name}.${block.label}` : block.name;
    if (componentIdx.has(id)) continue;

    componentIdx.set(id, components.length);
    components.push({
      localID: id,
      name: block.name,
      label: block.label || undefined,
      moduleID: '',
      dataFlowEdgesTo: [],
    });
  }

  // Second pass: for each top-level component block, find references to other
  // component IDs in its body tokens (at any nesting depth).
  const edgeSet = new Set<string>();

  for (const block of blocks) {
    if (block.depth !== 0 || block.name.split('.').length < 2) continue;

    const srcID = block.label ? `${block.name}.${block.label}` : block.name;
    const srcIdx = componentIdx.get(srcID);
    if (srcIdx === undefined) continue;

    // Collect traversals from the entire subtree rooted at this block.
    // Since extractBlocks records all blocks with their bodyTokens, we gather
    // tokens from all blocks that are descendants of this top-level block
    // (i.e. those that were declared while this block was on the stack — we
    // approximate by re-scanning all tokens inside our bodyTokens, which
    // already contains the full body text).
    const traversals = findTraversals(block.bodyTokens);

    for (const chain of traversals) {
      // Try each prefix of the chain to find a matching component ID.
      for (let prefixLen = 2; prefixLen <= chain.length; prefixLen++) {
        const candidateID = chain.slice(0, prefixLen).join('.');
        const dstIdx = componentIdx.get(candidateID);
        if (dstIdx !== undefined && dstIdx !== srcIdx) {
          // Determine edge direction from the field accessed after the component ID.
          const field = chain[prefixLen]; // may be undefined
          const isPush = field !== undefined && PUSH_FIELDS.has(field);

          // Push: A → B (A sends data to B's receiver/input).
          // Pull: B → A (B provides data to A; reference is reversed).
          const fromIdx = isPush ? srcIdx : dstIdx;
          const toIdx = isPush ? dstIdx : srcIdx;

          const fromID = components[fromIdx].localID;
          const toID = components[toIdx].localID;
          const edgeKey = `${fromID}|${toID}`;

          if (!edgeSet.has(edgeKey)) {
            edgeSet.add(edgeKey);
            components[fromIdx].dataFlowEdgesTo.push(toID);
          }
          break; // Longest-prefix match: stop once we found a component.
        }
      }
    }
  }

  return components;
}
