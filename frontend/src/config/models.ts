export interface ModelOption {
  id: string;
  name: string;
  provider: 'openrouter' | 'gemini' | 'openai' | 'anthropic' | 'ollama' | 'ninerouter' | 'custom';
  description: string;
  isFree?: boolean;
}

export const AVAILABLE_MODELS: ModelOption[] = [
  // OpenRouter Free Models
  {
    id: 'openrouter/free',
    name: 'OpenRouter Auto-Free (تخصیص هوشمند رایگان)',
    provider: 'openrouter',
    description: 'Auto-routes to the best available free model on OpenRouter',
    isFree: true,
  },
  {
    id: 'qwen/qwen3.8-27b:free',
    name: 'Qwen 3.8 27B (Free)',
    provider: 'openrouter',
    description: 'Advanced 27B reasoning and code model (100% Free)',
    isFree: true,
  },
  {
    id: 'google/gemma-4-31b-it:free',
    name: 'Google Gemma 4 31B (Free)',
    provider: 'openrouter',
    description: 'Google Gemma 4 31B instruction model (100% Free)',
    isFree: true,
  },
  {
    id: 'nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free',
    name: 'Nvidia Nemotron 3 Reasoning (Free)',
    provider: 'openrouter',
    description: 'Nvidia reasoning model for auditing & critique (100% Free)',
    isFree: true,
  },
  {
    id: 'deepseek/deepseek-chat',
    name: 'DeepSeek V3 (Chat / Coding)',
    provider: 'openrouter',
    description: 'Ultra-fast precise code synthesizer by DeepSeek',
  },
  {
    id: 'deepseek/deepseek-r1',
    name: 'DeepSeek R1 (Reasoning)',
    provider: 'openrouter',
    description: 'Flagship open reasoning model for deep audits',
  },
  {
    id: 'meta-llama/llama-3.3-70b-instruct',
    name: 'Llama 3.3 70B Instruct',
    provider: 'openrouter',
    description: 'Meta flagship 70B architecture powerhouse',
  },
  {
    id: 'google/gemini-2.5-flash',
    name: 'Google Gemini 2.5 Flash',
    provider: 'openrouter',
    description: 'Ultra-fast low-latency generation',
  },
  {
    id: 'anthropic/claude-3.5-sonnet',
    name: 'Claude 3.5 Sonnet',
    provider: 'anthropic',
    description: 'Anthropic state-of-the-art coding intelligence',
  },
  {
    id: 'openai/gpt-4o',
    name: 'OpenAI GPT-4o',
    provider: 'openai',
    description: 'OpenAI flagship multimodal intelligence',
  },
  {
    id: 'flash_5',
    name: 'NineRouter: Flash 5 (Gemini Lite)',
    provider: 'ninerouter',
    description: 'Fast synthesis via local 9Router combo',
    isFree: true,
  },
  {
    id: 'mimo',
    name: 'NineRouter: MiMo (v2.5)',
    provider: 'ninerouter',
    description: 'High-capability coder via local 9Router combo',
    isFree: true,
  },
  {
    id: 'bunny',
    name: 'NineRouter: Bunny (Space Bunny)',
    provider: 'ninerouter',
    description: 'Specialized red-team review & critique via local 9Router combo',
    isFree: true,
  },
  {
    id: 'nemotron',
    name: 'NineRouter: Nemotron (Nvidia Super)',
    provider: 'ninerouter',
    description: 'Nvidia Nemotron model via local 9Router gateway',
    isFree: true,
  },
  {
    id: 'nemo_qwen',
    name: 'NineRouter: Nemo Qwen (Nvidia Ultra)',
    provider: 'ninerouter',
    description: 'Nvidia Nemo Qwen high-reasoning model via local 9Router gateway',
    isFree: true,
  },
  {
    id: 'ollama/deepseek-r1:8b',
    name: 'Ollama: DeepSeek-R1 (Local)',
    provider: 'ollama',
    description: 'Local offline DeepSeek reasoning',
  },
];

export interface SwarmPreset {
  id: string;
  name: string;
  nameFa: string;
  description: string;
  models: {
    architect: string;
    coder: string;
    critic: string;
    security: string;
    arbiter: string;
  };
}

export const SWARM_PRESETS: SwarmPreset[] = [
  {
    id: 'ninerouter-developers',
    name: '🚀 NineRouter Developers Team (تیم دولوپر ها)',
    nameFa: 'تیم دولوپر ها (ناین‌روتر ۵ نفره)',
    description: 'تیم ۵ نفره متشکل از Nemo Qwen، Nemotron، Bunny، Flash 5 و MiMo بر بستر ناین‌روتر.',
    models: {
      architect: 'nemo_qwen',
      coder: 'mimo',
      critic: 'bunny',
      security: 'nemotron',
      arbiter: 'flash_5',
    },
  },
  {
    id: 'ninerouter-combo',
    name: '🔥 NineRouter 3-Model Combo (Flash 5 + MiMo + Bunny)',
    nameFa: 'تیم کمبو ناین‌روتر (فلش ۵ + مای‌مو + بانی)',
    description: 'Flash 5 Architect & Security, MiMo Coder, Bunny Critic & Arbiter via local 9Router gateway.',
    models: {
      architect: 'flash_5',
      coder: 'mimo',
      critic: 'bunny',
      security: 'flash_5',
      arbiter: 'bunny',
    },
  },
  {
    id: 'openrouter-smart-free',
    name: '⚡ OpenRouter 100% Free Team (Recommended)',
    nameFa: 'تیم تمام رایگان اوپن‌روتر (پیشنهادی)',
    description: 'All 5 agents run via openrouter/free zero-cost gateway.',
    models: {
      architect: 'openrouter/free',
      coder: 'openrouter/free',
      critic: 'openrouter/free',
      security: 'openrouter/free',
      arbiter: 'openrouter/free',
    },
  },
  {
    id: 'openrouter-free-auto',
    name: '🚀 Free Specialist Models',
    nameFa: 'تیم تخصصی مدل‌های رایگان',
    description: 'Gemma 4 Architect, Qwen 3.8 Coder & Security, Nemotron Reasoning Critic.',
    models: {
      architect: 'google/gemma-4-31b-it:free',
      coder: 'qwen/qwen3.8-27b:free',
      critic: 'nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free',
      security: 'qwen/qwen3.8-27b:free',
      arbiter: 'openrouter/free',
    },
  },
  {
    id: 'deepseek-full',
    name: '🧠 DeepSeek V3 + R1 Swarm',
    nameFa: 'تیم تخصصی دیپ‌سیک (کدنویسی + استنتاج)',
    description: 'DeepSeek R1 for architecture and critique, DeepSeek V3 for coding.',
    models: {
      architect: 'deepseek/deepseek-r1',
      coder: 'deepseek/deepseek-chat',
      critic: 'deepseek/deepseek-r1',
      security: 'deepseek/deepseek-chat',
      arbiter: 'deepseek/deepseek-r1',
    },
  },
  {
    id: 'hybrid-power',
    name: '🔥 Flagship Hybrid Team',
    nameFa: 'تیم هایبرید قدرتمند (Llama 70B + DeepSeek)',
    description: 'Llama 3.3 70B Architect, DeepSeek Coder & Critic, Gemini Security.',
    models: {
      architect: 'meta-llama/llama-3.3-70b-instruct',
      coder: 'deepseek/deepseek-chat',
      critic: 'deepseek/deepseek-r1',
      security: 'google/gemini-2.5-flash',
      arbiter: 'deepseek/deepseek-r1',
    },
  },
];
