/**
 * WebhookTab component displays webhook configuration for a DAG.
 *
 * @module features/dags/components/dag-details
 */
import { Check, Copy, Link, Terminal, Webhook, WebhookOff } from 'lucide-react';
import React, { useState } from 'react';
import { components } from '../../../../api/v2/schema';
import { Button } from '../../../../components/ui/button';

type WebhookConfig = components['schemas']['WebhookConfig'];

interface WebhookTabProps {
  fileName: string;
  webhook?: WebhookConfig;
}

function WebhookTab({ fileName, webhook }: WebhookTabProps) {
  const [copiedUrl, setCopiedUrl] = useState(false);
  const [copiedToken, setCopiedToken] = useState(false);
  const [copiedCurl, setCopiedCurl] = useState(false);

  // Construct webhook URL
  const webhookUrl = `${window.location.origin}/api/v2/webhooks/${encodeURIComponent(fileName)}`;

  // Handle copy functionality
  const handleCopy = async (text: string, setCopied: (v: boolean) => void) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard API might not be available
    }
  };

  // Generate example curl command
  const tokenForCurl = webhook?.token || '<YOUR_TOKEN>';
  const curlExample = `curl -X POST "${webhookUrl}" \\
  -H "Authorization: Bearer ${tokenForCurl}" \\
  -H "Content-Type: application/json" \\
  -d '{"key": "value"}'`;

  // No webhook configured
  if (!webhook) {
    return (
      <div className="flex flex-col items-center justify-center py-8">
        <WebhookOff className="h-8 w-8 text-muted-foreground mb-3" />
        <p className="text-sm text-muted-foreground mb-2">
          Webhook is not configured for this DAG
        </p>
        <p className="text-xs text-muted-foreground mb-3">
          Add a{' '}
          <code className="bg-muted px-1 py-0.5 rounded text-xs border">
            webhook
          </code>{' '}
          section to your DAG spec to enable webhooks:
        </p>
        <pre className="p-2 bg-muted rounded text-xs text-left font-mono border">
          {`webhook:
  enabled: true
  token: "\${WEBHOOK_SECRET}"`}
        </pre>
      </div>
    );
  }

  // Webhook disabled
  if (!webhook.enabled) {
    return (
      <div className="flex flex-col items-center justify-center py-8">
        <WebhookOff className="h-8 w-8 text-muted-foreground mb-3" />
        <p className="text-sm text-muted-foreground mb-2">
          Webhook is configured but disabled
        </p>
        <p className="text-xs text-muted-foreground">
          Set{' '}
          <code className="bg-muted px-1 py-0.5 rounded text-xs border">
            enabled: true
          </code>{' '}
          in your webhook configuration to enable it.
        </p>
      </div>
    );
  }

  // Webhook enabled
  return (
    <div className="space-y-4 py-2">
      {/* Status */}
      <div className="flex items-center gap-2">
        <div className="p-1.5 rounded bg-[rgba(107,168,107,0.15)] border border-[#6ba86b]">
          <Webhook className="h-4 w-4 text-[#5a8a5a]" />
        </div>
        <span className="text-sm font-medium text-[#5a8a5a]">
          Webhook Enabled
        </span>
        <span className="text-xs text-muted-foreground">
          This DAG can be triggered via HTTP
        </span>
      </div>

      {/* Endpoint URL */}
      <div>
        <div className="flex items-center mb-1.5">
          <Link className="h-3.5 w-3.5 mr-1 text-muted-foreground" />
          <span className="text-xs font-semibold text-foreground/90">
            Endpoint
          </span>
        </div>
        <div className="flex items-center gap-2">
          <div className="flex-1 p-2 bg-muted rounded border text-xs font-mono overflow-x-auto">
            <span className="text-muted-foreground">POST</span>{' '}
            <span className="text-foreground">{webhookUrl}</span>
          </div>
          <Button
            variant="outline"
            size="sm"
            className="h-8 w-8 p-0 shrink-0"
            onClick={() => handleCopy(webhookUrl, setCopiedUrl)}
          >
            {copiedUrl ? (
              <Check className="h-3.5 w-3.5" />
            ) : (
              <Copy className="h-3.5 w-3.5" />
            )}
          </Button>
        </div>
      </div>

      {/* Token */}
      {webhook.token && (
        <div>
          <div className="flex items-center mb-1.5">
            <Terminal className="h-3.5 w-3.5 mr-1 text-muted-foreground" />
            <span className="text-xs font-semibold text-foreground/90">
              Bearer Token
            </span>
          </div>
          <div className="flex items-center gap-2">
            <div className="flex-1 p-2 bg-muted rounded border text-xs font-mono overflow-x-auto">
              <span className="text-foreground">{webhook.token}</span>
            </div>
            <Button
              variant="outline"
              size="sm"
              className="h-8 w-8 p-0 shrink-0"
              onClick={() => handleCopy(webhook.token!, setCopiedToken)}
            >
              {copiedToken ? (
                <Check className="h-3.5 w-3.5" />
              ) : (
                <Copy className="h-3.5 w-3.5" />
              )}
            </Button>
          </div>
        </div>
      )}

      {/* Example cURL */}
      <div>
        <div className="flex items-center justify-between mb-1.5">
          <div className="flex items-center">
            <Terminal className="h-3.5 w-3.5 mr-1 text-muted-foreground" />
            <span className="text-xs font-semibold text-foreground/90">
              Example Request
            </span>
          </div>
          <Button
            variant="outline"
            size="sm"
            className="h-6 px-2"
            onClick={() => handleCopy(curlExample, setCopiedCurl)}
          >
            {copiedCurl ? (
              <Check className="h-3 w-3 mr-1" />
            ) : (
              <Copy className="h-3 w-3 mr-1" />
            )}
            <span className="text-xs">Copy</span>
          </Button>
        </div>
        <pre className="p-2 bg-muted rounded border text-xs font-mono overflow-x-auto whitespace-pre-wrap text-foreground">
          {curlExample}
        </pre>
      </div>

      {/* Info */}
      <div className="flex items-start gap-2 p-2 bg-muted/50 rounded border text-xs text-muted-foreground">
        <span className="font-medium text-foreground/90">Note:</span>
        <span>
          The request body is passed to the DAG as the{' '}
          <code className="bg-muted px-1 py-0.5 rounded border text-foreground">
            WEBHOOK_PAYLOAD
          </code>{' '}
          environment variable.
        </span>
      </div>
    </div>
  );
}

export default WebhookTab;
