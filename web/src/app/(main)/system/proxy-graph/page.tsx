"use client";

import * as React from "react";
import { toast } from "sonner";
import { PlusIcon, RefreshCwIcon, Trash2Icon, ShieldIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { http } from "@/lib/api";

type ProxyNode = {
  id: string;
  url: string;
  type: string;
  asn: string;
  active: boolean;
  score: number;
};

type RotationPolicy = "round_robin" | "asn_diverse" | "least_used";

const POLICY_LABELS: Record<RotationPolicy, string> = {
  round_robin: "Round Robin",
  asn_diverse: "ASN Diverse",
  least_used: "Least Used",
};

const emptyForm = { id: "", url: "", type: "datacenter", asn: "" };

export default function ProxyGraphPage() {
  const [nodes, setNodes] = React.useState<ProxyNode[]>([]);
  const [policy, setPolicy] = React.useState<RotationPolicy>("round_robin");
  const [form, setForm] = React.useState(emptyForm);
  const [adding, setAdding] = React.useState(false);
  const [healthLoading, setHealthLoading] = React.useState(false);
  const [policyLoading, setPolicyLoading] = React.useState(false);

  const loadNodes = React.useCallback(() => {
    http<{ nodes: ProxyNode[] }>("/proxygraph/nodes")
      .then((r) => setNodes(r.nodes ?? []))
      .catch(() => toast.error("Failed to load proxy nodes"));
  }, []);

  const loadConfig = React.useCallback(() => {
    http<{ policy: RotationPolicy }>("/proxygraph/config")
      .then((r) => setPolicy(r.policy ?? "round_robin"))
      .catch(() => {});
  }, []);

  React.useEffect(() => {
    loadNodes();
    loadConfig();
  }, [loadNodes, loadConfig]);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.id.trim() || !form.url.trim()) {
      toast.error("ID and URL are required");
      return;
    }
    setAdding(true);
    try {
      await http("/proxygraph/nodes", {
        method: "POST",
        body: JSON.stringify(form),
      });
      toast.success("Proxy node added");
      setForm(emptyForm);
      loadNodes();
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to add proxy node");
    } finally {
      setAdding(false);
    }
  };

  const handleRemove = async (id: string) => {
    try {
      await http(`/proxygraph/nodes/${encodeURIComponent(id)}`, { method: "DELETE" });
      toast.success(`Removed ${id}`);
      loadNodes();
    } catch {
      toast.error("Failed to remove proxy node");
    }
  };

  const handleHealthCheck = async () => {
    setHealthLoading(true);
    try {
      const r = await http<{ nodes: ProxyNode[] }>("/proxygraph/health");
      setNodes(r.nodes ?? []);
      toast.success("Health check complete");
    } catch {
      toast.error("Health check failed");
    } finally {
      setHealthLoading(false);
    }
  };

  const handlePolicyChange = async (val: string) => {
    setPolicyLoading(true);
    try {
      await http("/proxygraph/config", {
        method: "PUT",
        body: JSON.stringify({ policy: val }),
      });
      setPolicy(val as RotationPolicy);
      toast.success("Policy updated");
    } catch {
      toast.error("Failed to update policy");
    } finally {
      setPolicyLoading(false);
    }
  };

  return (
    <div className="flex flex-col gap-6 p-6 max-w-4xl mx-auto">
      <div className="flex items-center gap-2">
        <ShieldIcon className="h-5 w-5" />
        <h1 className="text-xl font-semibold">Proxy Graph</h1>
      </div>

      {/* Rotation policy */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Rotation Policy</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center gap-4">
          <Select
            value={policy}
            onValueChange={handlePolicyChange}
            disabled={policyLoading}
          >
            <SelectTrigger className="w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {Object.entries(POLICY_LABELS).map(([val, label]) => (
                <SelectItem key={val} value={val}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <span className="text-xs text-muted-foreground">
            Controls how the next egress proxy is selected per tool call.
          </span>
        </CardContent>
      </Card>

      {/* Add proxy node */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Add Proxy Node</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleAdd} className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-1">
              <Label htmlFor="pg-id">ID</Label>
              <Input
                id="pg-id"
                placeholder="proxy-1"
                value={form.id}
                onChange={(e) => setForm((f) => ({ ...f, id: e.target.value }))}
              />
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="pg-url">URL</Label>
              <Input
                id="pg-url"
                placeholder="http://proxy.host:8080"
                value={form.url}
                onChange={(e) => setForm((f) => ({ ...f, url: e.target.value }))}
              />
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="pg-type">Type</Label>
              <Select
                value={form.type}
                onValueChange={(v) => setForm((f) => ({ ...f, type: v }))}
              >
                <SelectTrigger id="pg-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="datacenter">Datacenter</SelectItem>
                  <SelectItem value="residential">Residential</SelectItem>
                  <SelectItem value="tor">TOR</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-1">
              <Label htmlFor="pg-asn">ASN</Label>
              <Input
                id="pg-asn"
                placeholder="AS1234"
                value={form.asn}
                onChange={(e) => setForm((f) => ({ ...f, asn: e.target.value }))}
              />
            </div>
            <div className="col-span-2 flex justify-end">
              <Button type="submit" disabled={adding} size="sm">
                <PlusIcon className="h-4 w-4 mr-1" />
                {adding ? "Adding…" : "Add Node"}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      {/* Node list */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle className="text-sm font-medium">
            Proxy Nodes ({nodes.length})
          </CardTitle>
          <Button
            variant="outline"
            size="sm"
            onClick={handleHealthCheck}
            disabled={healthLoading}
          >
            <RefreshCwIcon className="h-4 w-4 mr-1" />
            {healthLoading ? "Checking…" : "Health Check"}
          </Button>
        </CardHeader>
        <CardContent>
          {nodes.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No proxy nodes configured.
            </p>
          ) : (
            <div className="flex flex-col gap-2">
              {nodes.map((n) => (
                <div
                  key={n.id}
                  className="flex items-center justify-between border rounded-md px-3 py-2"
                >
                  <div className="flex flex-col gap-0.5">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-mono font-medium">{n.id}</span>
                      <Badge variant={n.active ? "default" : "secondary"}>
                        {n.active ? "active" : "inactive"}
                      </Badge>
                      <Badge variant="outline">{n.type}</Badge>
                    </div>
                    <span className="text-xs text-muted-foreground">
                      {n.url}
                      {n.asn ? ` · ${n.asn}` : ""} · score {n.score.toFixed(2)}
                    </span>
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleRemove(n.id)}
                    title="Remove"
                  >
                    <Trash2Icon className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
