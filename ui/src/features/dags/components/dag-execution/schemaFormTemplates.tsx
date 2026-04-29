import { Templates } from '@rjsf/shadcn';
import type {
  BaseInputTemplateProps,
  FormContextType,
  RJSFSchema,
  StrictRJSFSchema,
  TemplatesType,
} from '@rjsf/utils';
import React from 'react';

import { cn } from '@/lib/utils';

function SchemaBaseInputTemplate<
  T = any,
  S extends StrictRJSFSchema = RJSFSchema,
  F extends FormContextType = any,
>(props: BaseInputTemplateProps<T, S, F>) {
  const BaseInputTemplate =
    Templates.BaseInputTemplate as unknown as React.ComponentType<
      BaseInputTemplateProps<T, S, F>
    >;

  return (
    <BaseInputTemplate {...props} className={cn('bg-card', props.className)} />
  );
}

export const schemaFormTemplates: Partial<TemplatesType> = {
  BaseInputTemplate: SchemaBaseInputTemplate,
};
