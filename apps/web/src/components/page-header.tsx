import { Link, useRouter } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";

interface PageHeaderProps {
  title: string;
  description?: React.ReactNode;
  backTo?: string;
  backParams?: Record<string, string>;
  /** Use browser history.back() instead of a fixed link */
  useBack?: boolean;
  /** Slot for badges rendered after the title */
  badges?: React.ReactNode;
  /** Slot for action buttons on the right */
  actions?: React.ReactNode;
}

export function PageHeader({
  title,
  description,
  backTo,
  backParams,
  useBack,
  badges,
  actions,
}: PageHeaderProps) {
  const router = useRouter();

  return (
    <div className="flex items-center gap-3">
      {useBack ? (
        <button
          type="button"
          onClick={() => router.history.back()}
          className="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground"
          aria-label="Go back"
        >
          <ArrowLeft className="h-5 w-5" />
        </button>
      ) : backTo ? (
        <Link
          to={backTo}
          params={backParams as any}
          className="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground"
          aria-label="Go back"
        >
          <ArrowLeft className="h-5 w-5" />
        </Link>
      ) : null}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
          {badges}
        </div>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}
