import { Skeleton } from "@/components/ui/skeleton";

/**
 * Full-page loading skeleton for route pages.
 * Pass `variant` to choose between different shapes.
 */
export function LoadingScreen({ variant = "default" }: { variant?: "default" | "detail" }) {
  if (variant === "detail") {
    return (
      <div className="animate-fade-in space-y-4">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-4 w-96" />
        <div className="mt-6 grid grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
        <Skeleton className="mt-4 h-[300px]" />
      </div>
    );
  }

  return (
    <div className="animate-fade-in space-y-4">
      <Skeleton className="h-8 w-48" />
      <div className="mt-4 space-y-3">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-16 w-full" />
        ))}
      </div>
    </div>
  );
}
