import { Modal } from "../components/ui";
import { ConnectionPanel } from "../components/ConnectionPanel";

/** Connection settings reachable even while no snapshot can be loaded. */
export function ConnectionModal({ onClose }: { onClose: () => void }) {
  return (
    <Modal
      title="连接设置"
      subtitle="选择桌面端连接的服务端；保存后重启应用生效"
      width={660}
      onClose={onClose}
      footer={
        <button type="button" className="btn ghost" onClick={onClose}>
          关闭
        </button>
      }
    >
      <ConnectionPanel />
    </Modal>
  );
}
