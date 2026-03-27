import type { LucideIcon } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

interface StatCardProps {
  label: string;
  value: string | React.ReactNode;
  icon?: LucideIcon;
  iconColor?: string;
  loading?: boolean;
}

export function StatCard({ label, value, icon: Icon, iconColor, loading }: StatCardProps) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{label}</CardTitle>
        {Icon && <Icon className={`h-4 w-4 ${iconColor ?? "text-muted-foreground"}`} />}
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-8 w-16" />
        ) : (
          <div className="text-2xl font-bold">{value}</div>
        )}
      </CardContent>
    </Card>
  );
}

/** Compact stat card without the CardHeader structure (used in detail pages) */
export function StatCardCompact({
  label,
  value,
  loading,
}: {
  label: string;
  value: string | React.ReactNode;
  loading?: boolean;
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-xs text-muted-foreground">{label}</p>
        {loading ? (
          <Skeleton className="mt-1 h-7 w-16" />
        ) : (
          <p className="mt-1 text-lg font-bold">{value}</p>
        )}
      </CardContent>
    </Card>
  );
}
