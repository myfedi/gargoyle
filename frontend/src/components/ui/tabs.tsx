import { cn } from "@/lib/utils";

type TabsProps<T extends string> = {
  value: T;
  onValueChange: (value: T) => void;
  items: Array<{
    value: T;
    label: string;
    disabled?: boolean;
  }>;
  className?: string;
};

export function Tabs<T extends string>({ value, onValueChange, items, className }: TabsProps<T>) {
  return (
    <div className={cn("inline-flex rounded-lg bg-secondary p-1", className)} role="tablist">
      {items.map((item) => {
        const isActive = item.value === value;
        return (
          <button
            key={item.value}
            type="button"
            role="tab"
            aria-selected={isActive}
            disabled={item.disabled}
            onClick={() => onValueChange(item.value)}
            className={cn(
              "rounded-md px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors disabled:cursor-not-allowed disabled:opacity-50",
              isActive && "bg-background text-foreground shadow-sm",
            )}
          >
            {item.label}
          </button>
        );
      })}
    </div>
  );
}
