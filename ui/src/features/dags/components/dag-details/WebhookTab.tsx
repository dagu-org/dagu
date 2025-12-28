/**
 * WebhookTab component displays webhook configuration for a DAG.
 *
 * @module features/dags/components/dag-details
 */
import { Check, Copy, Terminal, Webhook, WebhookOff } from 'lucide-react';
import React, { useState } from 'react';
import { components } from '../../../../api/v2/schema';
import { Button } from '../../../../components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '../../../../components/ui/card';

type WebhookConfig = components['schemas']['WebhookConfig'];

interface WebhookTabProps {
  fileName: string;
  webhook?: WebhookConfig;
}

interface CopyButtonProps {
  text: string;
  copied: boolean;
  onCopy: () => void;
  label?: string;
}

function CopyButton({ copied, onCopy, label }: CopyButtonProps) {
  return (
    <Button
      variant="ghost"
      size="sm"
      className="h-6 px-2 text-muted-foreground hover:text-foreground"
      onClick={onCopy}
    >
      {copied ? (
        <Check className="h-3 w-3" />
      ) : (
        <Copy className="h-3 w-3" />
      )}
      {label && <span className="ml-1 text-xs">{label}</span>}
    </Button>
  );
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
  -d '{"dagRunId": "my-unique-id", "payload": {"key": "value"}}'`;

  // No webhook configured
  if (!webhook) {
    return (
      <Card className="max-w-xl gap-0 py-0">
        <CardHeader className="pb-2 px-4 pt-3">
          <div className="flex items-center gap-2">
            <WebhookOff className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-sm">Webhook Not Configured</CardTitle>
          </div>
          <CardDescription className="text-xs">
            Add a webhook section to your DAG spec to enable HTTP triggers
          </CardDescription>
        </CardHeader>
        <CardContent className="px-4 pb-3 pt-0">
          <pre className="px-3 py-2 bg-accent rounded-md text-xs font-mono border">
            {`webhook:
  enabled: true
  token: "\${WEBHOOK_SECRET}"`}
          </pre>
        </CardContent>
      </Card>
    );
  }

  // Webhook disabled
  if (!webhook.enabled) {
    return (
      <Card className="max-w-xl gap-0 py-0">
        <CardHeader className="px-4 py-3">
          <div className="flex items-center gap-2">
            <WebhookOff className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-sm">Webhook Disabled</CardTitle>
          </div>
          <CardDescription className="text-xs">
            Set <code className="bg-muted px-1 rounded">enabled: true</code> to
            activate
          </CardDescription>
        </CardHeader>
      </Card>
    );
  }

  // Webhook enabled
  return (
    <div className="space-y-3 max-w-2xl">
      {/* Status Card */}
      <Card className="gap-0 py-0">
        <CardHeader className="pb-0 px-4 pt-3">
          <div className="flex items-center gap-2">
            <Webhook className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-sm">Webhook</CardTitle>
          </div>
        </CardHeader>
        <CardContent className="px-4 pb-3 pt-2 space-y-3">
          {/* Endpoint */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs text-muted-foreground">Endpoint</span>
              <CopyButton
                text={webhookUrl}
                copied={copiedUrl}
                onCopy={() => handleCopy(webhookUrl, setCopiedUrl)}
              />
            </div>
            <div className="px-3 py-2 bg-accent rounded-md text-xs font-mono border overflow-x-auto">
              <span className="text-muted-foreground">POST</span>{' '}
              <span>{webhookUrl}</span>
            </div>
          </div>

          {/* Token */}
          {webhook.token && (
            <div>
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs text-muted-foreground">
                  Bearer Token
                </span>
                <CopyButton
                  text={webhook.token}
                  copied={copiedToken}
                  onCopy={() => handleCopy(webhook.token!, setCopiedToken)}
                />
              </div>
              <div className="px-3 py-2 bg-accent rounded-md text-xs font-mono border overflow-x-auto">
                {webhook.token}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Example Card */}
      <Card className="gap-0 py-0">
        <CardHeader className="pb-0 px-4 pt-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Terminal className="h-3.5 w-3.5 text-muted-foreground" />
              <CardTitle className="text-sm">Example Request</CardTitle>
            </div>
            <CopyButton
              text={curlExample}
              copied={copiedCurl}
              onCopy={() => handleCopy(curlExample, setCopiedCurl)}
              label="Copy"
            />
          </div>
        </CardHeader>
        <CardContent className="px-4 pb-3 pt-2">
          <pre className="px-3 py-2 bg-accent rounded-md text-xs font-mono border overflow-x-auto whitespace-pre-wrap">
            {curlExample}
          </pre>
          <ul className="mt-2 text-xs text-muted-foreground space-y-1">
            <li>
              <code className="bg-accent px-1 rounded-md border">payload</code>{' '}
              is available as{' '}
              <code className="bg-accent px-1 rounded-md border">WEBHOOK_PAYLOAD</code>{' '}
              env var.
            </li>
            <li>
              <code className="bg-accent px-1 rounded-md border">dagRunId</code>{' '}
              (optional) can be used as an idempotency key.
            </li>
          </ul>
        </CardContent>
      </Card>
    </div>
  );
}

export default WebhookTab;
