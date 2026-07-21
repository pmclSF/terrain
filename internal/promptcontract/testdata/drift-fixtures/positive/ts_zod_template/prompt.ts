import OpenAI from "openai";
// BINDING: prompt references ${email}, absent from Ticket → DRIFT
export const p = (t: {ticketId: string; subject: string}) =>
  `You are support. Reply to ${t.email} about ${t.subject}.`;
