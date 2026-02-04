import { useRef, useMemo } from 'react'
import { useFrame } from '@react-three/fiber'
import * as THREE from 'three'

export default function StarField() {
  const starsRef = useRef()
  const nebulasRef = useRef()

  // 生成 15000 个星点
  const [positions, colors, sizes] = useMemo(() => {
    const positions = new Float32Array(15000 * 3)
    const colors = new Float32Array(15000 * 3)
    const sizes = new Float32Array(15000)

    for (let i = 0; i < 15000; i++) {
      // 球形分布
      const radius = 100 + Math.random() * 400
      const theta = Math.random() * Math.PI * 2
      const phi = Math.acos(2 * Math.random() - 1)

      positions[i * 3] = radius * Math.sin(phi) * Math.cos(theta)
      positions[i * 3 + 1] = radius * Math.sin(phi) * Math.sin(theta)
      positions[i * 3 + 2] = radius * Math.cos(phi)

      // 随机颜色（蓝白黄红）
      const colorType = Math.random()
      if (colorType < 0.6) {
        // 蓝白色（主序星）
        colors[i * 3] = 0.7 + Math.random() * 0.3
        colors[i * 3 + 1] = 0.8 + Math.random() * 0.2
        colors[i * 3 + 2] = 1
      } else if (colorType < 0.85) {
        // 黄色（类太阳恒星）
        colors[i * 3] = 1
        colors[i * 3 + 1] = 0.9 + Math.random() * 0.1
        colors[i * 3 + 2] = 0.6 + Math.random() * 0.2
      } else {
        // 红色（红巨星）
        colors[i * 3] = 1
        colors[i * 3 + 1] = 0.3 + Math.random() * 0.2
        colors[i * 3 + 2] = 0.2 + Math.random() * 0.2
      }

      // 随机大小
      sizes[i] = Math.random() * 2 + 0.5
    }

    return [positions, colors, sizes]
  }, [])

  // 星云粒子
  const nebulaParticles = useMemo(() => {
    const particles = new Float32Array(3000 * 3)
    const nebulaColors = new Float32Array(3000 * 3)

    for (let i = 0; i < 3000; i++) {
      // 集中在某些区域形成星云
      const cluster = Math.floor(Math.random() * 3)
      const clusterCenter = [
        [150, 100, -200],
        [-180, -120, 150],
        [100, -150, 180]
      ][cluster]

      particles[i * 3] = clusterCenter[0] + (Math.random() - 0.5) * 100
      particles[i * 3 + 1] = clusterCenter[1] + (Math.random() - 0.5) * 100
      particles[i * 3 + 2] = clusterCenter[2] + (Math.random() - 0.5) * 100

      // 星云颜色（紫色、品红、青色）
      if (cluster === 0) {
        nebulaColors[i * 3] = 0.42
        nebulaColors[i * 3 + 1] = 0.18
        nebulaColors[i * 3 + 2] = 0.71
      } else if (cluster === 1) {
        nebulaColors[i * 3] = 0.85
        nebulaColors[i * 3 + 1] = 0.27
        nebulaColors[i * 3 + 2] = 0.94
      } else {
        nebulaColors[i * 3] = 0.02
        nebulaColors[i * 3 + 1] = 0.71
        nebulaColors[i * 3 + 2] = 0.83
      }
    }

    return [particles, nebulaColors]
  }, [])

  // 动画：缓慢旋转
  useFrame((state) => {
    if (starsRef.current) {
      starsRef.current.rotation.y += 0.0001
      starsRef.current.rotation.x = Math.sin(state.clock.elapsedTime * 0.1) * 0.05
    }
    if (nebulasRef.current) {
      nebulasRef.current.rotation.y -= 0.0002
      nebulasRef.current.rotation.x = Math.cos(state.clock.elapsedTime * 0.08) * 0.03
    }
  })

  return (
    <>
      {/* 星点 */}
      <points ref={starsRef}>
        <bufferGeometry>
          <bufferAttribute
            attach="attributes-position"
            count={positions.length / 3}
            array={positions}
            itemSize={3}
          />
          <bufferAttribute
            attach="attributes-color"
            count={colors.length / 3}
            array={colors}
            itemSize={3}
          />
          <bufferAttribute
            attach="attributes-size"
            count={sizes.length}
            array={sizes}
            itemSize={1}
          />
        </bufferGeometry>
        <pointsMaterial
          size={1.5}
          vertexColors
          transparent
          opacity={0.8}
          sizeAttenuation
          blending={THREE.AdditiveBlending}
          depthWrite={false}
        />
      </points>

      {/* 星云粒子 */}
      <points ref={nebulasRef}>
        <bufferGeometry>
          <bufferAttribute
            attach="attributes-position"
            count={nebulaParticles[0].length / 3}
            array={nebulaParticles[0]}
            itemSize={3}
          />
          <bufferAttribute
            attach="attributes-color"
            count={nebulaParticles[1].length / 3}
            array={nebulaParticles[1]}
            itemSize={3}
          />
        </bufferGeometry>
        <pointsMaterial
          size={8}
          vertexColors
          transparent
          opacity={0.15}
          sizeAttenuation
          blending={THREE.AdditiveBlending}
          depthWrite={false}
        />
      </points>

      {/* 环境光 */}
      <ambientLight intensity={0.2} />
      <pointLight position={[0, 0, 0]} intensity={0.5} color="#6b2fb5" />
    </>
  )
}
