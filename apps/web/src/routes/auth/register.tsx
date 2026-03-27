import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/auth/register")({
  beforeLoad: () => {
    throw redirect({ to: "/auth/login" });
  },
  component: () => null,
});
