import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Sparkles } from "lucide-react";
import { api } from "../../api";
import type { SelfUpdateCheck } from "../../types";
import { Button, Card, Pill, SkeletonRows } from "../ui";
import { SelfUpdateDialog } from "./SelfUpdateDialog";

export function SelfUpdateCard({ index = 2 }: { index?: number }) {
  const { t } = useTranslation();
  const [check, setCheck] = useState<SelfUpdateCheck>();
  const [dialogOpen, setDialogOpen] = useState(false);

  useEffect(() => {
    api.selfUpdateCheck().then(setCheck).catch(() => {});
  }, []);

  return (
    <Card index={index} title={t("selfupdate.title")} icon={Sparkles}
      action={check && (check.available
        ? <Pill tone="warn">{t("selfupdate.available")}</Pill>
        : <Pill tone="ok">{t("selfupdate.upToDate")}</Pill>)}>
      {!check ? (
        <SkeletonRows rows={3} />
      ) : (
        <div className="flex flex-col gap-2.5">
          <div className="flex items-baseline justify-between gap-4">
            <span className="text-small text-muted">{t("selfupdate.current")}</span>
            <span className="font-mono text-small font-medium">{check.current}</span>
          </div>

          {check.available && (
            <>
              <div className="flex items-baseline justify-between gap-4">
                <span className="text-small text-muted">{t("selfupdate.latest")}</span>
                <span className="font-mono text-small font-semibold text-warn">{check.latest}</span>
              </div>
              <div>
                <Button size="sm" onClick={() => setDialogOpen(true)}>{t("selfupdate.update")}</Button>
              </div>
            </>
          )}
        </div>
      )}

      <SelfUpdateDialog open={dialogOpen} onClose={() => setDialogOpen(false)} initialCheck={check} />
    </Card>
  );
}
