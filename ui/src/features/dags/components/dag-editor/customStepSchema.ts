import type { components } from '../../../../api/v1/schema';
import { parse as parseYaml } from 'yaml';
import { dereferenceSchema, type JSONSchema } from '@/lib/schema-utils';

export interface EditorCustomStepTypeHint {
  name: string;
  targetType: string;
  description?: string;
  inputSchema: JSONSchema;
}

export interface ExtractCustomStepTypesResult {
  ok: boolean;
  stepTypes: EditorCustomStepTypeHint[];
}

const customStepTypeNamePattern = /^[A-Za-z][A-Za-z0-9_-]*$/;
const localCustomSchemaDefinitionsKey = 'customStepTypeInputSchemas';

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function cloneJson<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function escapeJsonPointerSegment(segment: string): string {
  return segment.replace(/~/g, '~0').replace(/\//g, '~1');
}

function rewriteInternalRefs(value: unknown, basePointer: string): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => rewriteInternalRefs(item, basePointer));
  }
  if (!isRecord(value)) {
    return value;
  }

  const rewritten: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) {
    if (key === '$ref' && typeof item === 'string') {
      if (item === '#') {
        rewritten[key] = `#${basePointer}`;
      } else if (item.startsWith('#/')) {
        rewritten[key] = `#${basePointer}${item.slice(1)}`;
      } else {
        rewritten[key] = item;
      }
      continue;
    }
    rewritten[key] = rewriteInternalRefs(item, basePointer);
  }

  return rewritten;
}

export function toInheritedCustomStepTypeHints(
  editorHints?: components['schemas']['DAGEditorHints']
): EditorCustomStepTypeHint[] {
  const stepTypes: EditorCustomStepTypeHint[] = [];

  for (const hint of editorHints?.inheritedCustomStepTypes ?? []) {
    if (!hint?.name || !hint?.targetType || !isRecord(hint.inputSchema)) {
      continue;
    }

    const name = hint.name.trim();
    if (!customStepTypeNamePattern.test(name)) {
      continue;
    }

    stepTypes.push({
      name,
      targetType: hint.targetType.trim(),
      description: hint.description?.trim() || undefined,
      inputSchema: cloneJson(hint.inputSchema as JSONSchema),
    });
  }

  return stepTypes;
}

export function extractLocalCustomStepTypeHints(
  yamlContent: string
): ExtractCustomStepTypesResult {
  if (!yamlContent.trim()) {
    return { ok: true, stepTypes: [] };
  }

  let document: unknown;
  try {
    document = parseYaml(yamlContent);
  } catch {
    return { ok: false, stepTypes: [] };
  }

  if (!isRecord(document)) {
    return { ok: true, stepTypes: [] };
  }

  const stepTypesValue = document.step_types;
  if (!isRecord(stepTypesValue)) {
    return { ok: true, stepTypes: [] };
  }

  const stepTypes: EditorCustomStepTypeHint[] = [];
  for (const [rawName, rawDef] of Object.entries(stepTypesValue)) {
    if (!isRecord(rawDef)) {
      continue;
    }

    const name = rawName.trim();
    const targetType =
      typeof rawDef.type === 'string' ? rawDef.type.trim() : '';
    const description =
      typeof rawDef.description === 'string'
        ? rawDef.description.trim() || undefined
        : undefined;

    if (!customStepTypeNamePattern.test(name) || !targetType) {
      continue;
    }
    if (!isRecord(rawDef.input_schema)) {
      continue;
    }

    stepTypes.push({
      name,
      targetType,
      description,
      inputSchema: cloneJson(rawDef.input_schema as JSONSchema),
    });
  }

  return { ok: true, stepTypes };
}

export function mergeCustomStepTypeHints(
  inherited: EditorCustomStepTypeHint[],
  local: EditorCustomStepTypeHint[]
): EditorCustomStepTypeHint[] {
  const merged = new Map<string, EditorCustomStepTypeHint>();

  for (const hint of inherited) {
    merged.set(hint.name.trim(), hint);
  }
  for (const hint of local) {
    merged.set(hint.name.trim(), hint);
  }

  return Array.from(merged.values()).sort((left, right) =>
    left.name.localeCompare(right.name)
  );
}

export function buildAugmentedDAGSchema(
  baseSchema: JSONSchema,
  stepTypes: EditorCustomStepTypeHint[]
): JSONSchema {
  if (stepTypes.length === 0) {
    return baseSchema;
  }

  const augmented = cloneJson(baseSchema);
  const definitions = augmented.definitions;
  const stepSchema = definitions?.step;
  const executorTypeSchema = definitions?.executorType;

  if (!definitions || !stepSchema || !executorTypeSchema) {
    return baseSchema;
  }

  const customDefinitions: Record<string, JSONSchema> = {};
  const customStepRules: JSONSchema[] = [];
  const customTypeNames: string[] = [];
  const customTypeDescriptions: string[] = [];

  for (const stepType of stepTypes) {
    const escapedName = escapeJsonPointerSegment(stepType.name);
    const definitionPointer = `/definitions/${localCustomSchemaDefinitionsKey}/definitions/${escapedName}`;
    customDefinitions[stepType.name] = rewriteInternalRefs(
      cloneJson(stepType.inputSchema),
      definitionPointer
    ) as JSONSchema;

    customStepRules.push({
      if: {
        properties: { type: { const: stepType.name } },
        required: ['type'],
      },
      then: {
        properties: {
          config: {
            $ref: `#${definitionPointer}`,
          },
        },
      },
    });

    customTypeNames.push(stepType.name);
    customTypeDescriptions.push(
      stepType.description ||
        `Custom step type expanding to ${stepType.targetType}.`
    );
  }

  definitions[localCustomSchemaDefinitionsKey] = {
    definitions: customDefinitions,
  };

  stepSchema.allOf = [...(stepSchema.allOf ?? []), ...customStepRules];

  executorTypeSchema.anyOf = [
    ...(executorTypeSchema.anyOf?.slice(0, 1) ?? []),
    {
      type: 'string',
      enum: customTypeNames,
      enumDescriptions: customTypeDescriptions,
      description:
        'Custom step type declared in step_types or inherited from base config.',
    },
    ...(executorTypeSchema.anyOf?.slice(1) ?? []),
  ];

  return dereferenceSchema(augmented);
}
