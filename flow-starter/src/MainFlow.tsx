import { useCallback } from "react";
import {
  ReactFlow,
  Controls,
  addEdge,
  Connection,
  useNodesState,
  useEdgesState,
  Background,
} from "@xyflow/react";

import "@xyflow/react/dist/style.css";

import TextNode from "./TextNode";
import ResultNode from "./ResultNode";
import UppercaseNode from "./UppercaseNode";

const nodeTypes = {
  text: TextNode,
  result: ResultNode,
  uppercase: UppercaseNode,
};

const initNodes = [
  {
    id: "1",
    type: "text",
    data: {
      text: "hello",
    },
    position: { x: -100, y: -50 },
  },
  {
    id: "1a",
    type: "uppercase",
    data: {},
    position: { x: 100, y: 0 },
  },

  {
    id: "2",
    type: "text",
    data: {
      text: "world",
    },
    position: { x: 0, y: 100 },
  },

  {
    id: "3",
    type: "result",
    data: {},
    position: { x: 300, y: 50 },
  },
];

const initEdges = [
  {
    id: "e1-1a",
    source: "1",
    target: "1a",
  },
  {
    id: "e1a-3",
    source: "1a",
    target: "3",
  },
  {
    id: "e2-3",
    source: "2",
    target: "3",
  },
];

const CustomNodeFlow = () => {
  const [nodes, setNodes, onNodesChange] = useNodesState(initNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initEdges);

  const onConnect = useCallback(
    (connection: Connection) => setEdges((eds) => addEdge(connection, eds)),
    [setEdges]
  );

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      onConnect={onConnect}
      nodeTypes={nodeTypes}
      colorMode="system"
      fitView
    >
      <Controls />
      <Background />
    </ReactFlow>
  );
};

export default CustomNodeFlow;

