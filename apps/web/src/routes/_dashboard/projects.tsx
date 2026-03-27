import { createFileRoute, Link } from "@tanstack/react-router";
import { ChevronRight, FolderKanban, Plus } from "lucide-react";
import { useState } from "react";
import { EmptyState } from "@/components/empty-state";
import { LoadingScreen } from "@/components/loading-screen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useCreateProject, useProjects } from "@/hooks/use-projects";

export const Route = createFileRoute("/_dashboard/projects")({
  component: ProjectsPage,
});

function ProjectsPage() {
  const { data: projects, isLoading } = useProjects();
  const createProject = useCreateProject();
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    await createProject.mutateAsync({ name, description });
    setName("");
    setDescription("");
    setShowCreate(false);
  }

  if (isLoading) return <LoadingScreen />;

  return (
    <div>
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Projects</h1>
          <p className="text-sm text-muted-foreground">Manage your projects</p>
        </div>
        <Button onClick={() => setShowCreate(!showCreate)}>
          <Plus className="h-4 w-4" /> New Project
        </Button>
      </div>

      {showCreate && (
        <Card className="mt-4">
          <CardContent className="pt-4">
            <form onSubmit={handleCreate} className="flex gap-3">
              <Input
                placeholder="Project name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                className="max-w-xs"
              />
              <Input
                placeholder="Description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="flex-1"
              />
              <Button type="submit" disabled={createProject.isPending}>
                {createProject.isPending ? "..." : "Create"}
              </Button>
            </form>
          </CardContent>
        </Card>
      )}

      {!projects?.length ? (
        <div className="mt-8">
          <EmptyState
            icon={FolderKanban}
            message="No projects yet. Create your first project to get started."
            actionLabel="New Project"
            onAction={() => setShowCreate(true)}
          />
        </div>
      ) : (
        <div className="mt-6 grid gap-4">
          {projects.map((p) => (
            <Link key={p.id} to="/projects/$id" params={{ id: p.id }} className="block">
              <Card className="cursor-pointer transition-colors hover:bg-accent/50">
                <CardContent className="flex items-center justify-between p-4">
                  <div className="flex items-center gap-3">
                    <div className="flex h-9 w-9 items-center justify-center rounded-md bg-primary/10">
                      <FolderKanban className="h-4 w-4 text-primary" />
                    </div>
                    <div>
                      <p className="font-medium">{p.name}</p>
                      <p className="text-sm text-muted-foreground">
                        {p.description || "No description"}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <Badge variant="secondary">{new Date(p.created_at).toLocaleDateString()}</Badge>
                    <ChevronRight className="h-4 w-4 text-muted-foreground" />
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
