/**
 * The neighbourhood of an item, as a small graph.
 *
 * The graph is drawn with a plain force simulation. It is not the
 * heavyweight graph library: a dump item can hold thousands of claims,
 * and the graph of the card holds at most a few dozen nodes, which is
 * small enough to lay out with a simple simulation.
 *
 * The layout is computed once per item, in an effect, so that the
 * render is not doing the physics.
 */

import { useEffect, useMemo, useRef, useState } from "react";

import type { EntityView } from "../view";
import { toGraphData } from "../view";

/**
 * The props of the graph.
 */
export interface EntityGraphProps {
  /** The item whose neighbourhood is drawn. */
  readonly item: EntityView;
  /** The number of nodes at most. */
  readonly maxNodes?: number;
  /** The size of the drawing, in pixels. */
  readonly width?: number;
  /** The size of the drawing, in pixels. */
  readonly height?: number;
}

/**
 * A node of the graph, with its position.
 */
interface PlacedNode {
  readonly id: string;
  readonly label: string;
  x: number;
  y: number;
  vx: number;
  vy: number;
}

/**
 * An edge of the graph.
 */
interface PlacedEdge {
  readonly from: string;
  readonly to: string;
}

/**
 * The graph of the neighbourhood of an item.
 */
export function EntityGraph({
  item,
  maxNodes = 30,
  width = 520,
  height = 260,
}: EntityGraphProps) {
  const [placed, setPlaced] = useState<{ nodes: PlacedNode[]; edges: PlacedEdge[] }>({
    nodes: [],
    edges: [],
  });
  const raf = useRef<number | null>(null);

  const graph = useMemo(() => toGraphData(item, maxNodes), [item, maxNodes]);

  useEffect(() => {
    if (graph.nodes.length === 0) {
      setPlaced({ nodes: [], edges: [] });
      return;
    }

    // The nodes start on a circle, so that the simulation does not
    // start from a single point.
    const nodes: PlacedNode[] = graph.nodes.map((node, index) => {
      const angle = (index / Math.max(graph.nodes.length, 1)) * Math.PI * 2;
      const radius = Math.min(width, height) * 0.3;
      return {
        id: node.id,
        label: node.label,
        x: width / 2 + Math.cos(angle) * radius,
        y: height / 2 + Math.sin(angle) * radius,
        vx: 0,
        vy: 0,
      };
    });

    const edges: PlacedEdge[] = graph.edges
      .filter((edge) => edge.from !== edge.to)
      .map((edge) => ({ from: edge.from, to: edge.to }));

    // The simulation is stepped in an animation frame, so that a big
    // graph does not block the paint.
    let step = 0;
    const totalSteps = 120;

    const tick = () => {
      if (step >= totalSteps) {
        setPlaced({ nodes, edges });
        return;
      }
      step += 1;
      layoutStep(nodes, edges, width, height);
      raf.current = requestAnimationFrame(tick);
    };

    raf.current = requestAnimationFrame(tick);

    return () => {
      if (raf.current !== null) {
        cancelAnimationFrame(raf.current);
        raf.current = null;
      }
    };
  }, [graph, width, height]);

  if (graph.nodes.length <= 1) {
    return (
      <p className="text-xs text-muted-foreground">
        This item has no entity to show next to it.
      </p>
    );
  }

  const byId = new Map(placed.nodes.map((node) => [node.id, node]));

  return (
    <div>
      <svg
        width="100%"
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        aria-label={`The neighbourhood of ${item.id}`}
        className="rounded border border-border bg-muted/40"
      >
        {placed.edges.map((edge, index) => {
          const from = byId.get(edge.from);
          const to = byId.get(edge.to);
          if (!from || !to) {
            return null;
          }
          return (
            <line
              key={`${edge.from}-${edge.to}-${index}`}
              x1={from.x}
              y1={from.y}
              x2={to.x}
              y2={to.y}
              stroke="currentColor"
              strokeOpacity={0.25}
              strokeWidth={1}
            />
          );
        })}

        {placed.nodes.map((node) => {
          const isCenter = node.id === item.id;
          return (
            <g key={node.id}>
              <circle
                cx={node.x}
                cy={node.y}
                r={isCenter ? 7 : 4.5}
                fill={isCenter ? "hsl(var(--primary))" : "hsl(var(--muted-foreground))"}
              >
                <title>{node.label}</title>
              </circle>
              {isCenter ? null : (
                <text
                  x={node.x}
                  y={node.y - 9}
                  textAnchor="middle"
                  className="fill-current text-[9px] opacity-70"
                >
                  {node.label}
                </text>
              )}
            </g>
          );
        })}
      </svg>

      <p className="mt-1 text-[0.65rem] text-muted-foreground">
        {graph.nodes.length} entities, {graph.edges.length} links
        {graph.nodes.length >= maxNodes ? " (the graph is capped)" : ""}
      </p>
    </div>
  );
}

/**
 * One step of the simulation.
 *
 * It is the same physics as `layout`, but a single step, so that the
 * animation can spread the steps over the frames.
 */
function layoutStep(nodes: PlacedNode[], edges: PlacedEdge[], width: number, height: number): void {
  const repulsion = 900;
  const springLength = 70;
  const springStrength = 0.05;
  const damping = 0.85;

  const byId = new Map(nodes.map((node) => [node.id, node]));

  for (let i = 0; i < nodes.length; i++) {
    for (let j = i + 1; j < nodes.length; j++) {
      const a = nodes[i];
      const b = nodes[j];
      const dx = b.x - a.x;
      const dy = b.y - a.y;
      const distanceSquared = dx * dx + dy * dy || 1;
      const distance = Math.sqrt(distanceSquared);
      const force = repulsion / distanceSquared;
      a.vx -= (dx / distance) * force;
      a.vy -= (dy / distance) * force;
      b.vx += (dx / distance) * force;
      b.vy += (dy / distance) * force;
    }
  }

  for (const edge of edges) {
    const a = byId.get(edge.from);
    const b = byId.get(edge.to);
    if (!a || !b) {
      continue;
    }
    const dx = b.x - a.x;
    const dy = b.y - a.y;
    const distance = Math.sqrt(dx * dx + dy * dy) || 1;
    const force = (distance - springLength) * springStrength;
    a.vx += (dx / distance) * force;
    a.vy += (dy / distance) * force;
    b.vx -= (dx / distance) * force;
    b.vy -= (dy / distance) * force;
  }

  const centreX = width / 2;
  const centreY = height / 2;
  for (const node of nodes) {
    node.vx += (centreX - node.x) * 0.01;
    node.vy += (centreY - node.y) * 0.01;
    node.vx *= damping;
    node.vy *= damping;
    node.x += node.vx;
    node.y += node.vy;
  }
}
