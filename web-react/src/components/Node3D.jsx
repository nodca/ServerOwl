import { useRef, useMemo } from 'react'
import { useFrame } from '@react-three/fiber'
import { Text } from '@react-three/drei'
import * as THREE from 'three'

// 创建纹理生成函数
function createTextures(type) {
  const canvas = document.createElement('canvas')
  canvas.width = 512
  canvas.height = 512
  const ctx = canvas.getContext('2d')

  // 法线贴图 canvas
  const normalCanvas = document.createElement('canvas')
  normalCanvas.width = 512
  normalCanvas.height = 512
  const normalCtx = normalCanvas.getContext('2d')

  if (type === 'star') {
    // 恒星纹理 - 更亮更鲜艳
    const gradient = ctx.createRadialGradient(256, 256, 0, 256, 256, 256)
    gradient.addColorStop(0, '#ffffff')
    gradient.addColorStop(0.2, '#ffeb3b')
    gradient.addColorStop(0.5, '#ff9800')
    gradient.addColorStop(0.8, '#ff5722')
    gradient.addColorStop(1, '#d32f2f')
    ctx.fillStyle = gradient
    ctx.fillRect(0, 0, 512, 512)

    // 太阳黑子 - 更明显
    for (let i = 0; i < 30; i++) {
      const x = Math.random() * 512
      const y = Math.random() * 512
      const radius = Math.random() * 20 + 8
      ctx.fillStyle = `rgba(100, 50, 0, ${Math.random() * 0.5 + 0.3})`
      ctx.beginPath()
      ctx.arc(x, y, radius, 0, Math.PI * 2)
      ctx.fill()
    }

    // 添加亮斑
    for (let i = 0; i < 15; i++) {
      const x = Math.random() * 512
      const y = Math.random() * 512
      const radius = Math.random() * 25 + 10
      ctx.fillStyle = `rgba(255, 255, 200, ${Math.random() * 0.3 + 0.1})`
      ctx.beginPath()
      ctx.arc(x, y, radius, 0, Math.PI * 2)
      ctx.fill()
    }

    // 法线贴图（平滑）
    normalCtx.fillStyle = '#8080ff'
    normalCtx.fillRect(0, 0, 512, 512)

  } else if (type === 'planet') {
    // 行星纹理 - 更鲜艳的蓝绿色
    const gradient = ctx.createRadialGradient(256, 256, 50, 256, 256, 256)
    gradient.addColorStop(0, '#64b5f6')
    gradient.addColorStop(0.4, '#2196f3')
    gradient.addColorStop(0.7, '#1976d2')
    gradient.addColorStop(1, '#0d47a1')
    ctx.fillStyle = gradient
    ctx.fillRect(0, 0, 512, 512)

    // 大陆 - 更亮的绿色
    ctx.fillStyle = '#4caf50'
    const continents = []
    for (let i = 0; i < 20; i++) {
      const x = Math.random() * 512
      const y = Math.random() * 512
      const width = Math.random() * 120 + 60
      const height = Math.random() * 100 + 50
      continents.push({ x, y, width, height })
      ctx.beginPath()
      ctx.ellipse(x, y, width, height, Math.random() * Math.PI, 0, Math.PI * 2)
      ctx.fill()
    }

    // 山脉 - 深绿色
    ctx.fillStyle = '#2e7d32'
    for (let i = 0; i < 15; i++) {
      const x = Math.random() * 512
      const y = Math.random() * 512
      const radius = Math.random() * 40 + 20
      ctx.beginPath()
      ctx.arc(x, y, radius, 0, Math.PI * 2)
      ctx.fill()
    }

    // 云层 - 更明显
    ctx.fillStyle = 'rgba(255, 255, 255, 0.6)'
    for (let i = 0; i < 40; i++) {
      const x = Math.random() * 512
      const y = Math.random() * 512
      const radius = Math.random() * 40 + 15
      ctx.beginPath()
      ctx.arc(x, y, radius, 0, Math.PI * 2)
      ctx.fill()
    }

    // 法线贴图（海洋平滑，大陆凹凸）
    normalCtx.fillStyle = '#8080ff' // 海洋平滑
    normalCtx.fillRect(0, 0, 512, 512)

    // 大陆凹凸
    continents.forEach(cont => {
      for (let i = 0; i < 50; i++) {
        const x = cont.x + (Math.random() - 0.5) * cont.width
        const y = cont.y + (Math.random() - 0.5) * cont.height
        const radius = Math.random() * 10 + 3
        normalCtx.fillStyle = `rgba(${100 + Math.random() * 100}, ${100 + Math.random() * 100}, 200, 0.5)`
        normalCtx.beginPath()
        normalCtx.arc(x, y, radius, 0, Math.PI * 2)
        normalCtx.fill()
      }
    })

  } else {
    // 死亡星球 - 更明显的陨石坑
    const gradient = ctx.createRadialGradient(256, 256, 50, 256, 256, 256)
    gradient.addColorStop(0, '#616161')
    gradient.addColorStop(0.5, '#424242')
    gradient.addColorStop(1, '#212121')
    ctx.fillStyle = gradient
    ctx.fillRect(0, 0, 512, 512)

    // 陨石坑 - 更深更明显
    const craters = []
    for (let i = 0; i < 50; i++) {
      const x = Math.random() * 512
      const y = Math.random() * 512
      const radius = Math.random() * 30 + 10
      craters.push({ x, y, radius })

      ctx.fillStyle = `rgba(0, 0, 0, ${Math.random() * 0.6 + 0.4})`
      ctx.beginPath()
      ctx.arc(x, y, radius, 0, Math.PI * 2)
      ctx.fill()

      // 陨石坑边缘高光
      ctx.fillStyle = `rgba(150, 150, 150, ${Math.random() * 0.3 + 0.1})`
      ctx.beginPath()
      ctx.arc(x - radius * 0.3, y - radius * 0.3, radius * 0.3, 0, Math.PI * 2)
      ctx.fill()
    }

    // 法线贴图（陨石坑凹陷）
    normalCtx.fillStyle = '#8080ff'
    normalCtx.fillRect(0, 0, 512, 512)

    craters.forEach(crater => {
      // 陨石坑凹陷
      const gradient = normalCtx.createRadialGradient(
        crater.x, crater.y, 0,
        crater.x, crater.y, crater.radius
      )
      gradient.addColorStop(0, '#4040a0')
      gradient.addColorStop(0.7, '#6060c0')
      gradient.addColorStop(1, '#8080ff')
      normalCtx.fillStyle = gradient
      normalCtx.beginPath()
      normalCtx.arc(crater.x, crater.y, crater.radius, 0, Math.PI * 2)
      normalCtx.fill()
    })
  }

  const colorTexture = new THREE.CanvasTexture(canvas)
  colorTexture.needsUpdate = true

  const normalTexture = new THREE.CanvasTexture(normalCanvas)
  normalTexture.needsUpdate = true

  return { colorTexture, normalTexture }
}

export default function Node3D({ position, node, onClick, isSelected }) {
  const meshRef = useRef()
  const glowRef = useRef()
  const atmosphereRef = useRef()
  const ringsRef = useRef()

  const celestialType = node.isMaster ? 'star' : (node.status === 'online' ? 'planet' : 'dead')
  const size = node.isMaster ? 3 : 2

  const textures = useMemo(() => createTextures(celestialType), [celestialType])

  useFrame((state) => {
    if (meshRef.current) {
      meshRef.current.rotation.y += 0.005

      if (celestialType === 'star') {
        const pulse = Math.sin(state.clock.elapsedTime * 2) * 0.1 + 1
        meshRef.current.scale.setScalar(size * pulse)
      }

      if (isSelected) {
        meshRef.current.rotation.x += 0.01
      }
    }

    if (atmosphereRef.current) {
      const breath = Math.sin(state.clock.elapsedTime * 1.5) * 0.1 + 1
      atmosphereRef.current.scale.setScalar(breath)
    }

    if (glowRef.current) {
      const glow = Math.sin(state.clock.elapsedTime * 1.2) * 0.2 + 0.8
      glowRef.current.scale.setScalar(size * 3 * glow)
    }

    if (ringsRef.current) {
      ringsRef.current.rotation.z += 0.003
    }
  })

  const color = celestialType === 'star' ? '#fbbf24' : (celestialType === 'planet' ? '#3b82f6' : '#ef4444')

  return (
    <group position={position} onClick={onClick}>
      {/* 外层光晕 */}
      <mesh ref={glowRef}>
        <sphereGeometry args={[1, 16, 16]} />
        <meshBasicMaterial
          color={color}
          transparent
          opacity={0.1}
          blending={THREE.AdditiveBlending}
        />
      </mesh>

      {/* 大气层 */}
      {celestialType === 'planet' && (
        <mesh ref={atmosphereRef} scale={1.15}>
          <sphereGeometry args={[size, 32, 32]} />
          <meshBasicMaterial
            color="#4a90e2"
            transparent
            opacity={0.2}
            blending={THREE.AdditiveBlending}
            side={THREE.BackSide}
          />
        </mesh>
      )}

      {/* 主天体 */}
      <mesh ref={meshRef} scale={size}>
        <sphereGeometry args={[1, 64, 64]} />
        <meshStandardMaterial
          map={textures.colorTexture}
          normalMap={textures.normalTexture}
          normalScale={[0.5, 0.5]}
          emissive={celestialType === 'star' ? '#ff9800' : (celestialType === 'planet' ? '#1976d2' : '#000000')}
          emissiveIntensity={celestialType === 'star' ? 2 : (celestialType === 'planet' ? 0.3 : 0)}
          roughness={celestialType === 'star' ? 0.1 : (celestialType === 'planet' ? 0.4 : 0.8)}
          metalness={celestialType === 'planet' ? 0.5 : 0.1}
          toneMapped={false}
        />
      </mesh>

      {/* 行星环 */}
      {node.isMaster && (
        <mesh ref={ringsRef} rotation={[Math.PI / 2.5, 0, 0]}>
          <ringGeometry args={[size * 1.5, size * 2, 64]} />
          <meshBasicMaterial
            color="#ffd700"
            transparent
            opacity={0.4}
            side={THREE.DoubleSide}
            blending={THREE.AdditiveBlending}
          />
        </mesh>
      )}

      {/* 标签 */}
      {isSelected && (
        <Text
          position={[0, size + 1, 0]}
          fontSize={0.4}
          color="#ffffff"
          anchorX="center"
          anchorY="middle"
          outlineWidth={0.05}
          outlineColor="#000000"
        >
          {node.hostname || node.id}
        </Text>
      )}

      {/* 点光源 */}
      {celestialType === 'star' && (
        <pointLight
          position={[0, 0, 0]}
          intensity={2}
          distance={20}
          color={color}
          decay={2}
        />
      )}
    </group>
  )
}
