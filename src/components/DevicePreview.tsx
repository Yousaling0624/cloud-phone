import type { CloudPhone } from "../mygo";
import { phoneState } from "../lib/format";
import { useScreenshot } from "../lib/useScreenshot";
import { Icon } from "./Icon";
import { Spinner } from "./ui";

/**
 * The live screen of the account's bound phone. It keeps the last good frame
 * when polling fails and says so instead of flashing an empty box.
 */
export function DevicePreview({
  accountId,
  phone,
  enabled,
}: {
  accountId: number;
  phone?: CloudPhone;
  enabled: boolean;
}) {
  const { src, error, loading, refresh } = useScreenshot(accountId, enabled && !!phone);

  return (
    <aside className="console-device" aria-label="云机实时画面">
      <div className="device-head">
        <span className="device-head-ico">
          <Icon name="phone" size={15} />
        </span>
        <span>云机画面</span>
        {phone && <span className={`device-state ${phoneState(phone.state).tone}`}>{phoneState(phone.state).label}</span>}
      </div>

      {!phone ? (
        <div className="device-empty">
          <Icon name="phone" size={22} />
          <span>未绑定云机，无法预览</span>
          <span className="device-empty-hint">在账号编辑里选择一台云机后即可看到实时画面。</span>
        </div>
      ) : (
        <div className={`device-frame${error ? " has-error" : ""}`}>
          {src ? (
            <img src={src} alt={`云机 ${phone.name} 的实时画面`} />
          ) : (
            <div className="device-empty">
              {loading ? <Spinner light /> : <Icon name="phone" size={22} />}
              <span>{loading ? "正在连接云机…" : "暂无画面"}</span>
            </div>
          )}
          {error && (
            <div className="device-error">
              <Icon name="alert" size={14} />
              <span>{error}</span>
              <button type="button" className="link-btn" onClick={refresh}>
                重试
              </button>
            </div>
          )}
        </div>
      )}

      <div className="device-meta">
        {phone ? (
          <>
            <span className="mono">{phone.name}</span>
            <span>·</span>
            <span>{phone.region}</span>
            <span>·</span>
            <span>{phone.image}</span>
          </>
        ) : (
          <span>控制台只展示已绑定云机的账号画面</span>
        )}
      </div>
      <div className="device-foot">
        画面每 3 秒刷新，只读；操作请使用外部 scrcpy / Escrcpy
      </div>
    </aside>
  );
}
