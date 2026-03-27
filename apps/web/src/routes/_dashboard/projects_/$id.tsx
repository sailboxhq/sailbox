import { createFileRoute } from "@tanstack/react-router";
import { AnimatedOutlet } from "@/components/animated-outlet";

export const Route = createFileRoute("/_dashboard/projects_/$id")({
  component: () => <AnimatedOutlet />,
});
