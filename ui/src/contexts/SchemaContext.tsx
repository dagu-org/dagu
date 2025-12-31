import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useMemo,
} from 'react';
import type { JSONSchema } from '@/lib/schema-utils';

const SCHEMA_URL =
  'https://raw.githubusercontent.com/dagu-org/dagu/main/schemas/dag.schema.json';

interface SchemaContextValue {
  schema: JSONSchema | null;
  loading: boolean;
  error: Error | null;
  reload: () => void;
}

const SchemaContext = createContext<SchemaContextValue>({
  schema: null,
  loading: true,
  error: null,
  reload: () => {},
});

// Cache the dereferenced schema at module level
let cachedSchema: JSONSchema | null = null;
let cachePromise: Promise<JSONSchema> | null = null;

/**
 * Resolves all $ref pointers in a JSON Schema
 * Only handles internal references (#/definitions/...)
 */
function dereferenceSchema(schema: JSONSchema): JSONSchema {
  const definitions = schema.definitions || {};

  function resolveRef(ref: string): JSONSchema {
    if (!ref.startsWith('#/definitions/')) {
      return {};
    }
    const defName = ref.replace('#/definitions/', '');
    return definitions[defName] || {};
  }

  function processNode(node: unknown): unknown {
    if (!node || typeof node !== 'object') {
      return node;
    }

    if (Array.isArray(node)) {
      return node.map(processNode);
    }

    const obj = node as Record<string, unknown>;

    // If node has $ref, replace with resolved reference
    if (typeof obj.$ref === 'string') {
      const resolved = resolveRef(obj.$ref);
      // Merge with any other properties (like description overrides)
      const { $ref, ...rest } = obj;
      return processNode({ ...resolved, ...rest });
    }

    // Process all properties recursively
    const result: Record<string, unknown> = {};
    for (const [key, value] of Object.entries(obj)) {
      result[key] = processNode(value);
    }
    return result;
  }

  return processNode(schema) as JSONSchema;
}

async function loadAndDereferenceSchema(): Promise<JSONSchema> {
  if (cachedSchema) {
    return cachedSchema;
  }

  if (cachePromise) {
    return cachePromise;
  }

  cachePromise = (async () => {
    const response = await fetch(SCHEMA_URL);
    if (!response.ok) {
      throw new Error(`Failed to fetch schema: ${response.statusText}`);
    }
    const rawSchema = await response.json();
    cachedSchema = dereferenceSchema(rawSchema as JSONSchema);
    return cachedSchema;
  })();

  return cachePromise;
}

export function SchemaProvider({ children }: { children: React.ReactNode }) {
  const [schema, setSchema] = useState<JSONSchema | null>(cachedSchema);
  const [loading, setLoading] = useState(!cachedSchema);
  const [error, setError] = useState<Error | null>(null);

  const loadSchema = async () => {
    setLoading(true);
    setError(null);
    try {
      const loadedSchema = await loadAndDereferenceSchema();
      setSchema(loadedSchema);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to load schema'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!cachedSchema) {
      loadSchema();
    }
  }, []);

  const reload = () => {
    cachedSchema = null;
    cachePromise = null;
    loadSchema();
  };

  const value = useMemo(
    () => ({
      schema,
      loading,
      error,
      reload,
    }),
    [schema, loading, error]
  );

  return (
    <SchemaContext.Provider value={value}>{children}</SchemaContext.Provider>
  );
}

export function useSchema() {
  return useContext(SchemaContext);
}
