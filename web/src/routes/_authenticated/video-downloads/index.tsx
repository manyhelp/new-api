import { createFileRoute, redirect } from '@tanstack/react-router'
import { VideoDownloads } from '@/features/video-downloads'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/video-downloads/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({ to: '/403' })
    }
  },
  component: VideoDownloads,
})
