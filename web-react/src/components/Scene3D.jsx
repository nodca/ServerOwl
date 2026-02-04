import { Canvas } from '@react-three/fiber'
import { OrbitControls, PerspectiveCamera } from '@react-three/drei'
import StarField from './StarField'
import Node3D from './Node3D'
import { useStore } from '../stores/useStore'

export default function Scene3D() {
  const nodes = useStore((state) => state.nodes)
  const selectedNode = useStore((state) => state.selectedNode)
  const selectNode = useStore((state) => state.selectNode)

  // 将节点排列成圆形
  const nodePositions = Array.from(nodes.values()).map((node, index) => {
    const angle = (index / nodes.size) * Math.PI * 2
    const radius = 30
    return {
      node,
      position: [
        Math.cos(angle) * radius,
        Math.sin(angle) * radius * 0.3,
        Math.sin(angle) * radius
      ]
    }
  })

  return (
    <div style={{ width: '100%', height: '100%', position: 'absolute', top: 0, left: 0 }}>
      <Canvas>
        <PerspectiveCamera makeDefault position={[0, 20, 80]} fov={60} />
        <OrbitControls
          enableDamping
          dampingFactor={0.05}
          rotateSpeed={0.5}
          minDistance={30}
          maxDistance={200}
          enablePan={false}
        />

        {/* 增强光照 */}
        <ambientLight intensity={0.8} />
        <directionalLight position={[10, 10, 5]} intensity={1.5} color="#ffffff" />
        <directionalLight position={[-10, -10, -5]} intensity={0.5} color="#4a90e2" />

        <StarField />

        {nodePositions.map(({ node, position }) => (
          <Node3D
            key={node.id}
            position={position}
            node={node}
            isSelected={selectedNode === node.id}
            onClick={() => selectNode(node.id)}
          />
        ))}

        {/* 连线 */}
        {nodePositions.length > 1 && (
          <lineSegments>
            <bufferGeometry>
              <bufferAttribute
                attach="attributes-position"
                count={nodePositions.length * 2}
                array={new Float32Array(
                  nodePositions.flatMap((item, i) => {
                    const next = nodePositions[(i + 1) % nodePositions.length]
                    return [...item.position, ...next.position]
                  })
                )}
                itemSize={3}
              />
            </bufferGeometry>
            <lineBasicMaterial color="#6b2fb5" transparent opacity={0.2} />
          </lineSegments>
        )}
      </Canvas>
    </div>
  )
}
