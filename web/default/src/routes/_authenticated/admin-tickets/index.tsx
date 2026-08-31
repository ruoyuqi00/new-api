import { createFileRoute } from '@tanstack/react-router'

import { TicketList } from '@/features/tickets'

export const Route = createFileRoute('/_authenticated/admin-tickets/')({
  component: () => <TicketList isAdmin />,
})
