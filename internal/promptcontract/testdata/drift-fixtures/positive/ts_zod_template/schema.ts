import { z } from "zod";
export const Ticket = z.object({ ticketId: z.string(), subject: z.string() }); // no `email`
