import { useMemo } from 'react';
import { useSchema } from '@/contexts/SchemaContext';
import {
  getSchemaAtPath,
  toPropertyInfo,
  getParentRequired,
  getSiblingProperties,
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
 */
export function useSchemaLookup(path: string[]): SchemaLookupResult {
  const { schema, loading, error } = useSchema();

  const result = useMemo(() => {
    if (!schema || path.length === 0) {
      return {
        propertyInfo: null,
        siblingProperties: [],
      };
    }

    const schemaAtPath = getSchemaAtPath(schema, path);
    const parentRequired = getParentRequired(schema, path);
    const currentKey = path[path.length - 1] ?? '';
    const propertyInfo = toPropertyInfo(schemaAtPath, currentKey, path, parentRequired);
    const siblingProperties = getSiblingProperties(schema, path);

    return {
      propertyInfo,
      siblingProperties,
    };
  }, [schema, path]);

  return {
    ...result,
    loading,
    error,
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
