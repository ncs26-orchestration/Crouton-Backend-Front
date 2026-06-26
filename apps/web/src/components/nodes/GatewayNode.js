import { Handle, Position } from "@xyflow/react";
export function GatewayNode({ data, selected }) {
    const size = Math.min(data.width, data.height);
    const isExclusive = data.gatewayType === "exclusive";
    return (<div className="relative flex items-center justify-center cursor-grab active:cursor-grabbing" style={{ width: data.width, height: data.height }}>
      <Handle type="target" position={Position.Left} className="!size-2 !bg-[var(--color-border-strong)] !border-2 !border-[var(--color-surface)]"/>
      <div className={`flex items-center justify-center bg-[var(--color-surface)] transition-all ${selected
            ? "border-2 border-[var(--color-brand)] shadow-[0_0_0_3px_var(--color-accent-bg)]"
            : "border border-[var(--color-border-strong)] shadow-stripe-ambient"}`} style={{
            width: size,
            height: size,
            transform: "rotate(45deg)",
            borderRadius: 6,
        }}>
        <span className={`font-mono text-[14px] ${selected ? "text-[var(--color-brand)]" : "text-[var(--color-fg)]"}`} style={{ transform: "rotate(-45deg)", fontWeight: 500 }}>
          {isExclusive ? "×" : "+"}
        </span>
      </div>
      <Handle type="source" position={Position.Right} className="!size-2 !bg-[var(--color-border-strong)] !border-2 !border-[var(--color-surface)]"/>
    </div>);
}
