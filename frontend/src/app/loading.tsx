import { Spinner } from "@/components/feedback/spinner";

export default function Loading() {
  return (
    <div className="text-muted-foreground fixed inset-0 z-50 flex items-center justify-center">
      <Spinner tamano="xl" />
    </div>
  );
}
