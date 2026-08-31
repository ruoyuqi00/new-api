import { createFileRoute } from '@tanstack/react-router'

import { TicketDetail } from '@/features/tickets'

export const Route = createFileRoute('/_authenticated/tickets/$ticketId')({
  component: RouteComponent,
})

function RouteComponent() {
  const { ticketId } = Route.useParams()
  return <TicketDetail ticketId={Number(ticketId)} />
}
