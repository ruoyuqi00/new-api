import { createFileRoute } from '@tanstack/react-router'

import { TicketDetail } from '@/features/tickets'

export const Route = createFileRoute('/_authenticated/admin-tickets/$ticketId')({
  component: RouteComponent,
})

function RouteComponent() {
  const { ticketId } = Route.useParams()
  return <TicketDetail ticketId={Number(ticketId)} isAdmin />
}
