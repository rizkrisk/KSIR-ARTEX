"use client";

import * as React from "react";

import { ShieldIcon } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const STORAGE_KEY_X64 = "c2_bin_x64";
const STORAGE_KEY_X86 = "c2_bin_x86";
const STORAGE_KEY_DIR = "c2_staging_dir";

function readLocal(key: string, fallback: string): string {
  try {
    return localStorage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

function writeLocal(key: string, value: string): void {
  try {
    localStorage.setItem(key, value);
  } catch {
    // ignore
  }
}

function statusBadge(path: string): React.ReactNode {
  if (!path.trim()) {
    return <span className="text-xs text-amber-500">Not configured</span>;
  }
  return <span className="text-xs text-emerald-500">Configured</span>;
}

export default function C2BridgePage() {
  const [binX64, setBinX64] = React.useState("");
  const [binX86, setBinX86] = React.useState("");
  const [stagingDir, setStagingDir] = React.useState("");
  const [loaded, setLoaded] = React.useState(false);

  React.useEffect(() => {
    setBinX64(readLocal(STORAGE_KEY_X64, "data/c2/implant_x64.exe"));
    setBinX86(readLocal(STORAGE_KEY_X86, "data/c2/implant_x86.exe"));
    setStagingDir(readLocal(STORAGE_KEY_DIR, "data/c2"));
    setLoaded(true);
  }, []);

  const save = () => {
    writeLocal(STORAGE_KEY_X64, binX64.trim());
    writeLocal(STORAGE_KEY_X86, binX86.trim());
    writeLocal(STORAGE_KEY_DIR, stagingDir.trim());
    toast.success("C2 bridge config saved (browser-local; set env vars ARTEX_C2_BIN_X64 / ARTEX_C2_BIN_X86 / ARTEX_C2_STAGING_DIR on the server)");
  };

  return (
    <div className="flex flex-1 flex-col gap-4 md:gap-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">C2 Bridge</h1>
        <p className="text-muted-foreground text-sm">
          Delivery-assist configuration for the Windows RAT implant. ARTEX stages and delivers the
          binary via an already-obtained shell, then hands off to the operator.
        </p>
      </div>

      <div className="columns-1 gap-4 md:gap-6 lg:columns-2">
        <Card className="mb-4 break-inside-avoid md:mb-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <ShieldIcon className="size-4" />
              Implant binary paths
            </CardTitle>
            <CardDescription>
              Local server-side paths to the RAT binaries. Override at runtime via environment
              variables: <code>ARTEX_C2_BIN_X64</code>, <code>ARTEX_C2_BIN_X86</code>,{" "}
              <code>ARTEX_C2_STAGING_DIR</code>. Paths stored here are browser-local reference
              only; the server reads the env vars directly.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="bin-x64" className="text-sm font-medium">
                  x64 binary path
                </Label>
                {loaded && statusBadge(binX64)}
              </div>
              <Input
                id="bin-x64"
                className="font-mono text-sm"
                placeholder="data/c2/implant_x64.exe"
                value={binX64}
                disabled={!loaded}
                onChange={(e) => setBinX64(e.target.value)}
              />
              <p className="text-muted-foreground text-xs">
                Server env var: <code>ARTEX_C2_BIN_X64</code>
              </p>
            </div>

            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="bin-x86" className="text-sm font-medium">
                  x86 binary path
                </Label>
                {loaded && statusBadge(binX86)}
              </div>
              <Input
                id="bin-x86"
                className="font-mono text-sm"
                placeholder="data/c2/implant_x86.exe"
                value={binX86}
                disabled={!loaded}
                onChange={(e) => setBinX86(e.target.value)}
              />
              <p className="text-muted-foreground text-xs">
                Server env var: <code>ARTEX_C2_BIN_X86</code>
              </p>
            </div>

            <div className="flex flex-col gap-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="staging-dir" className="text-sm font-medium">
                  Staging directory
                </Label>
                {loaded && statusBadge(stagingDir)}
              </div>
              <Input
                id="staging-dir"
                className="font-mono text-sm"
                placeholder="data/c2"
                value={stagingDir}
                disabled={!loaded}
                onChange={(e) => setStagingDir(e.target.value)}
              />
              <p className="text-muted-foreground text-xs">
                Server env var: <code>ARTEX_C2_STAGING_DIR</code>
              </p>
            </div>

            <div className="flex justify-end border-t pt-4">
              <Button onClick={save} disabled={!loaded}>
                Save
              </Button>
            </div>
          </CardContent>
        </Card>

        <Card className="mb-4 break-inside-avoid md:mb-6">
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <ShieldIcon className="size-4" />
              Agent tools
            </CardTitle>
            <CardDescription>
              Three tools are available to workers after binding in the agent tool catalog.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3 text-sm">
            <div className="border-border/60 rounded-md border p-3">
              <p className="font-mono font-medium">c2_stage_implant(arch)</p>
              <p className="text-muted-foreground mt-1 text-xs">
                Checks if the RAT binary is present locally for the given architecture. Returns the
                local path on success, error if missing.
              </p>
            </div>
            <div className="border-border/60 rounded-md border p-3">
              <p className="font-mono font-medium">c2_deliver_implant(vector, shell_url, target_path, arch)</p>
              <p className="text-muted-foreground mt-1 text-xs">
                Builds the delivery command for the chosen vector (echo_b64, certutil, bitsadmin,
                wget, curl, webshell_upload). Returns the command to execute via the shell/exec tool.
              </p>
            </div>
            <div className="border-border/60 rounded-md border p-3">
              <p className="font-mono font-medium">c2_await_operator(target_host, delivery_status)</p>
              <p className="text-muted-foreground mt-1 text-xs">
                Signals operator handoff. Returns the post-delivery handoff message with next steps.
                Agent work ends here; operator takes the session.
              </p>
            </div>
            <p className="text-muted-foreground text-xs">
              Bind tools via <b>System → Tools</b>. See{" "}
              <code>skills/c2-handoff/SKILL.md</code> for the full delivery doctrine.
            </p>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
