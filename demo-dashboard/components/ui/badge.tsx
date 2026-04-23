import * as React from "react";
import { cn } from "@/lib/utils";

type BadgeProps = React.HTMLAttributes<HTMLSpanElement> & {
  variant?: "default" | "success" | "destructive" | "secondary";
};

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold",
        variant === "default" && "bg-primary text-primary-foreground",
        variant === "success" && "bg-emerald-100 text-emerald-800",
        variant === "destructive" && "bg-red-100 text-red-800",
        variant === "secondary" && "bg-muted text-muted-foreground",
        className
      )}
      {...props}
    />
  );
}
