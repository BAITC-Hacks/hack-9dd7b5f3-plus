/** Stable application agent identity, independent of the underlying model. */
export const VOICE_AGENT_ID = "bagyt-voice-001";

/** Format legacy router explanations without exposing the model label in the UI. */
export function agentReason(reason: string, model?: string): string {
  if (!model) return reason;
  return reason.replaceAll(model, VOICE_AGENT_ID).replace(/^LLM /, "Voice Agent ");
}
