import { useMemo } from 'react';
import { useSchema } from '@/contexts/SchemaContext';
import {
  getSchemaAtPath,
  toPropertyInfo,
  getParentRequired,
  getSiblingProperties,
  type JSONSchema,
  type SchemaPropertyInfo,
} from '@/lib/schema-utils';

export interface SchemaLookupResult {
  propertyInfo: SchemaPropertyInfo | null;
  siblingProperties: string[];
  loading: boolean;
  error: Error | null;
}

/**
 * Hook to look up schema information for a given YAML path
 * @param path The YAML path to look up
 * @param yamlContent Optional YAML content for context-aware resolution of if-then conditionals
 * @param schemaOverride Optional document-specific schema override
 */
export function useSchemaLookup(
  path: string[],
  yamlContent?: string,
  schemaOverride?: JSONSchema | null
): SchemaLookupResult {
  const { schema, loading, error } = useSchema();
  const activeSchema = schemaOverride ?? schema;
  const hasSchemaOverride = !!schemaOverride;

  const result = useMemo(() => {
    if (!activeSchema || path.length === 0) {
      return {
        propertyInfo: null,
        siblingProperties: [],
      };
    }

    const schemaAtPath = getSchemaAtPath(activeSchema, path, yamlContent);
    const parentRequired = getParentRequired(activeSchema, path, yamlContent);
    const currentKey = path[path.length - 1] ?? '';
    const propertyInfo = toPropertyInfo(
      schemaAtPath,
      currentKey,
      path,
      parentRequired
    );
    const siblingProperties = getSiblingProperties(
      activeSchema,
      path,
      yamlContent
    );

    return {
      propertyInfo,
      siblingProperties,
    };
  }, [activeSchema, path, yamlContent]);

  return {
    ...result,
    loading: hasSchemaOverride ? false : loading,
    error: hasSchemaOverride ? null : error,
  };
}

/**
 * Hook to get all root-level properties from the schema
 */
export function useRootSchemaProperties(): {
  properties: string[];
  loading: boolean;
  error: Error | null;
} {
  const { schema, loading, error } = useSchema();

  const properties = useMemo(() => {
    if (!schema?.properties) {
      return [];
    }
    return Object.keys(schema.properties);
  }, [schema]);

  return { properties, loading, error };
}
