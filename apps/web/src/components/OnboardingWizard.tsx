import { useState, useRef, useEffect } from "react";
import { motion } from "framer-motion";
import { Check, ChevronRight, Loader2, X } from "lucide-react";

import { api } from "../lib/api";
import { useOrg } from "../contexts/OrgContext";

interface DeterministicQuestion {
  index: number;
  id: string;
  text: string;
  type: "text" | "single" | "multi";
  options?: string[];
  required: boolean;
  placeholder?: string;
}

interface Overview {
  name?: string;
  industry?: string;
  size?: string;
  regions?: string[];
  systems?: string[];
  processes?: string;
  compliance?: string[];
  languages?: string[];
}

interface Props {
  onComplete: (overview: Overview, projectId: string) => void;
  onSkip: () => void;
}

export function OnboardingWizard({ onComplete, onSkip }: Props) {
  const { activeOrg } = useOrg();
  const [questions, setQuestions] = useState<DeterministicQuestion[]>([]);
  const [currentStep, setCurrentStep] = useState(0);
  const [answers, setAnswers] = useState<Record<string, string | string[]>>({});
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [complete, setComplete] = useState(false);
  const [isSlowLoading, setIsSlowLoading] = useState(false);
  const slowTimeoutRef = useRef<number | null>(null);

  useEffect(() => {
    const fetchQuestions = async () => {
      setIsLoading(true);
      setError(null);
      setIsSlowLoading(false);

      slowTimeoutRef.current = window.setTimeout(() => {
        setIsSlowLoading(true);
      }, 5000);

      try {
        const resp = await api.getOnboardingQuestions();

        if (slowTimeoutRef.current) {
          clearTimeout(slowTimeoutRef.current);
        }

        if (resp.error) {
          setError(resp.error);
          setIsLoading(false);
          setIsSlowLoading(false);
          return;
        }

        if (resp.questions) {
          const castQuestions = resp.questions.map((q) => ({
            ...q,
            type: q.type as "text" | "single" | "multi",
          }));
          setQuestions(castQuestions);
        }
      } catch (e) {
        if (slowTimeoutRef.current) {
          clearTimeout(slowTimeoutRef.current);
        }
        setError(e instanceof Error ? e.message : "Failed to load onboarding");
      } finally {
        setIsLoading(false);
        setIsSlowLoading(false);
      }
    };

    fetchQuestions();

    return () => {
      if (slowTimeoutRef.current) {
        clearTimeout(slowTimeoutRef.current);
      }
    };
  }, []);

  const saveAnswer = (questionId: string, value: string | string[]) => {
    setAnswers((prev) => ({ ...prev, [questionId]: value }));
  };

  const handleNext = () => {
    const nextStep = currentStep + 1;
    if (nextStep >= questions.length) {
      handleComplete();
    } else {
      setCurrentStep(nextStep);
    }
  };

  const handleComplete = async () => {
    setIsSaving(true);
    setError(null);

    try {
      const orgName = (answers["name"] as string)?.trim();
      if (!orgName) {
        setError("Organisation name is required");
        setIsSaving(false);
        return;
      }

      const project = await api.createProject(activeOrg!.id, { name: orgName });

      const overview: Overview = {
        name: orgName,
        industry: answers["industry"] as string,
        size: answers["size"] as string,
        regions: answers["regions"] as string[],
        systems: answers["systems"] as string[],
        processes: answers["processes"] as string,
        compliance: answers["compliance"] as string[],
        languages: answers["languages"] as string[],
      };

      await api.updateProjectOverview(project.id, overview as unknown as Record<string, unknown>);

      setComplete(true);
      onComplete(overview, project.id);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save onboarding");
    } finally {
      setIsSaving(false);
    }
  };

  const handleSkip = async () => {
    const orgName = (answers["name"] as string)?.trim();
    
    if (orgName) {
      await handleComplete();
    } else {
      onSkip();
    }
  };

  const currentQuestion = questions[currentStep];

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-6"
    >
      <motion.div
        initial={{ opacity: 0, y: 20, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: 10, scale: 0.99 }}
        transition={{ type: "spring", stiffness: 300, damping: 26 }}
        className="relative max-w-lg w-full bg-[var(--color-surface)] border border-[var(--color-border)] rounded-xl shadow-stripe-deep overflow-hidden"
      >
        <button
          onClick={handleSkip}
          disabled={isLoading || isSaving}
          className="absolute top-4 right-4 text-[var(--color-fg-subtle)] hover:text-[var(--color-fg)] disabled:opacity-40"
          aria-label="Skip onboarding"
        >
          <X size={16} />
        </button>

        <div className="px-6 pt-6 pb-4 border-b border-[var(--color-border)]">
          <div className="text-[10px] uppercase tracking-widest text-[var(--color-fg-subtle)] mb-1">
            Onboarding
          </div>
          <h2 className="text-lg font-semibold text-[var(--color-fg)]">
            Tell us about your organisation
          </h2>
          <div className="flex items-center gap-2 mt-3">
            <div className="flex-1 h-1.5 bg-[var(--color-border)] rounded-full overflow-hidden">
              <div
                className="h-full bg-[var(--color-brand)] transition-all duration-300"
                style={{ width: complete ? "100%" : `${((currentStep + 1) / (questions.length || 1)) * 100}%` }}
              />
            </div>
            <span className="text-[11px] text-[var(--color-fg-subtle)] font-mono">
              {complete ? "Done" : `${currentStep + 1}/${questions.length}`}
            </span>
          </div>
        </div>

        <div className="px-6 py-5 min-h-[200px]">
          {isLoading && !currentQuestion && !error ? (
            <div className="flex flex-col items-center justify-center py-8">
              <Loader2 size={24} className="animate-spin text-[var(--color-brand)] mb-3" />
              {isSlowLoading ? (
                <p className="text-[13px] text-[var(--color-fg-muted)] text-center">
                  Taking longer than usual...
                </p>
              ) : (
                <p className="text-[13px] text-[var(--color-fg-muted)]">Loading questions...</p>
              )}
            </div>
          ) : isSaving ? (
            <div className="flex flex-col items-center justify-center py-8">
              <Loader2 size={24} className="animate-spin text-[var(--color-brand)] mb-3" />
              <p className="text-[13px] text-[var(--color-fg-muted)]">Creating project...</p>
            </div>
          ) : error ? (
            <div className="flex flex-col items-center justify-center py-6">
              <p className="text-[13px] text-red-500 text-center mb-4">{error}</p>
              <button
                onClick={() => setError(null)}
                className="text-[12px] px-4 py-2 rounded-md border border-[var(--color-border)] bg-[var(--color-surface-2)] text-[var(--color-fg)]"
              >
                Try Again
              </button>
            </div>
          ) : complete ? (
            <div className="text-center py-6">
              <div className="w-12 h-12 rounded-full bg-emerald-500/10 flex items-center justify-center mx-auto mb-3">
                <Check size={24} className="text-emerald-500" />
              </div>
              <p className="text-[15px] text-[var(--color-fg)] font-medium mb-1">✓ Complete</p>
              <p className="text-[12px] text-[var(--color-fg-muted)]">
                Organisation overview saved. You can edit it later from project settings.
              </p>
            </div>
          ) : currentQuestion ? (
            <QuestionRenderer
              question={currentQuestion}
              value={answers[currentQuestion.id]}
              onChange={(val) => saveAnswer(currentQuestion.id, val)}
              onNext={handleNext}
              isLoading={isLoading}
            />
          ) : null}
        </div>

        <div className="px-6 py-4 bg-[var(--color-surface-2)] border-t border-[var(--color-border)] flex items-center justify-between">
          <button
            onClick={handleSkip}
            disabled={isLoading || isSaving}
            className="text-xs text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] disabled:opacity-40"
          >
            Skip / Save & Exit
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
}

function QuestionRenderer({
  question,
  value,
  onChange,
  onNext,
  isLoading,
}: {
  question: DeterministicQuestion;
  value: string | string[] | undefined;
  onChange: (val: string | string[]) => void;
  onNext: () => void;
  isLoading: boolean;
}) {
  const textValue = typeof value === "string" ? value : "";
  const selectedSingle = typeof value === "string" ? value : "";
  const selectedMulti = Array.isArray(value) ? value : [];

  const handleSingleCustomChange = (vals: string[]) => {
    if (vals.length > 0) {
      onChange(`Other: ${vals.join(", ")}`);
    } else {
      onChange("");
    }
  };

  const handleMultiCustomChange = (vals: string[]) => {
    const filtered = selectedMulti.filter((o) => !o.startsWith("Other:"));
    if (vals.length > 0) {
      onChange([...filtered, ...vals.map(v => `Other: ${v}`)]);
    } else {
      onChange(filtered);
    }
  };

  const hasOtherSingle = selectedSingle === "Other" || selectedSingle.startsWith("Other:");
  const hasOtherMulti = selectedMulti.includes("Other") || selectedMulti.some((o) => o.startsWith("Other:"));

  const toggleMulti = (option: string) => {
    if (option === "None") {
      const hasNone = selectedMulti.includes("None");
      if (hasNone) {
        onChange(selectedMulti.filter((o) => o !== "None"));
      } else {
        onChange(["None"]);
      }
      return;
    }

    if (option === "Other") {
      const hasOther = selectedMulti.includes("Other");
      const otherCustom = selectedMulti.find((o) => o.startsWith("Other:"));
      
      if (hasOther) {
        onChange(selectedMulti.filter((o) => o !== "Other" && !o.startsWith("Other:")));
      } else if (otherCustom) {
        onChange(selectedMulti.filter((o) => !o.startsWith("Other:")));
      } else {
        onChange([...selectedMulti.filter(o => o !== "None"), "Other"]);
      }
      return;
    }
    
    onChange([...selectedMulti.filter(o => o !== "None" && o !== "Other" && !o.startsWith("Other:")), ...(selectedMulti.includes(option) ? [] : [option])]);
  };

  const isTextValid = question.type === "text" ? (textValue.trim() ? true : !question.required) : true;
  const isSingleValid = question.type === "single" ? (selectedSingle ? true : !question.required) : true;
  const isMultiValid = question.type === "multi" ? (selectedMulti.length > 0 ? true : !question.required) : true;

  return (
    <div>
      <p className="text-[15px] text-[var(--color-fg)] leading-relaxed mb-4">
        {question.text}
        {question.required && <span className="text-red-500 ml-1">*</span>}
      </p>

      {question.type === "text" && (
        <div>
          <textarea
            autoFocus
            value={textValue}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && e.preventDefault()}
            placeholder={question.placeholder}
            disabled={isLoading}
            rows={3}
            className="w-full bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-lg px-4 py-3 text-[14px] text-[var(--color-fg)] focus:outline-none focus:border-[var(--color-brand)] placeholder:text-[var(--color-fg-subtle)] disabled:opacity-50 resize-none"
          />
          <div className="flex justify-end mt-3">
            <button
              onClick={onNext}
              disabled={!isTextValid || isLoading}
              className="inline-flex items-center gap-1.5 text-xs px-4 py-2 rounded-md bg-[var(--color-brand)] text-white hover:brightness-110 disabled:opacity-40"
            >
              {isLoading ? <Loader2 size={12} className="animate-spin" /> : <><span>Continue</span><ChevronRight size={12} /></>}
            </button>
          </div>
        </div>
      )}

      {question.type === "single" && (
        <div className="space-y-2">
          <div className="grid grid-cols-2 gap-2">
            {question.options?.filter(o => o !== "Other").map((option) => (
              <button
                key={option}
                onClick={() => onChange(option)}
                disabled={isLoading}
                className={`w-full text-left px-4 py-2.5 rounded-lg border text-[13px] transition-colors ${
                  selectedSingle === option
                    ? "border-[var(--color-brand)] bg-[var(--color-accent-bg)] text-[var(--color-fg)]"
                    : "border-[var(--color-border)] text-[var(--color-fg)] hover:border-[var(--color-fg-subtle)]"
                }`}
              >
                {option}
              </button>
            ))}
          </div>
          {question.options?.includes("Other") && (
            <ChipInput
              values={selectedSingle.startsWith("Other:") 
                ? selectedSingle.replace("Other:", "").trim().split(",").map(s => s.trim()).filter(Boolean)
                : []
              }
              onChange={handleSingleCustomChange}
              placeholder="Type custom values and press Enter..."
              singleLine
            />
          )}
          <div className="flex justify-end mt-3">
            <button
              onClick={onNext}
              disabled={!isSingleValid || isLoading}
              className="inline-flex items-center gap-1.5 text-xs px-4 py-2 rounded-md bg-[var(--color-brand)] text-white hover:brightness-110 disabled:opacity-40"
            >
              {isLoading ? <Loader2 size={12} className="animate-spin" /> : <><span>Continue</span><ChevronRight size={12} /></>}
            </button>
          </div>
        </div>
      )}

      {question.type === "multi" && (
        <div className="space-y-3">
          <div className="flex flex-wrap gap-2">
            {question.options?.filter(o => o !== "Other" && o !== "None").map((option) => (
              <button
                key={option}
                onClick={() => toggleMulti(option)}
                disabled={isLoading}
                className={`px-3 py-1.5 rounded-full border text-[12px] transition-colors ${
                  selectedMulti.includes(option)
                    ? "border-[var(--color-brand)] bg-[var(--color-accent-bg)] text-[var(--color-fg)]"
                    : "border-[var(--color-border)] text-[var(--color-fg-muted)] hover:border-[var(--color-fg-subtle)]"
                }`}
              >
                {option}
              </button>
            ))}
          </div>
          <div className="flex flex-wrap gap-2 border-t border-[var(--color-border)] pt-3">
            <button
              onClick={() => toggleMulti("None")}
              disabled={isLoading}
              className={`px-3 py-1.5 rounded-full border text-[12px] transition-colors ${
                selectedMulti.includes("None")
                  ? "border-amber-500 bg-amber-500/10 text-amber-600"
                  : "border-[var(--color-border)] text-[var(--color-fg-muted)] hover:border-[var(--color-fg-subtle)]"
              }`}
            >
              None
            </button>
            {question.options?.includes("Other") && (
              <button
                onClick={() => toggleMulti("Other")}
                disabled={isLoading}
                className={`px-3 py-1.5 rounded-full border text-[12px] transition-colors ${
                  selectedMulti.includes("Other") || selectedMulti.some(o => o.startsWith("Other:"))
                    ? "border-[var(--color-brand)] bg-[var(--color-accent-bg)] text-[var(--color-fg)]"
                    : "border-[var(--color-border)] text-[var(--color-fg-muted)] hover:border-[var(--color-fg-subtle)]"
                }`}
              >
                Other
              </button>
            )}
          </div>
          {hasOtherMulti && (
            <ChipInput
              values={selectedMulti.filter(o => o.startsWith("Other:")).map(o => o.replace("Other:", "").trim())}
              onChange={handleMultiCustomChange}
              placeholder="Type custom values and press Enter..."
            />
          )}
          <div className="flex justify-end mt-3">
            <button
              onClick={onNext}
              disabled={!isMultiValid || isLoading}
              className="inline-flex items-center gap-1.5 text-xs px-4 py-2 rounded-md bg-[var(--color-brand)] text-white hover:brightness-110 disabled:opacity-40"
            >
              {isLoading ? <Loader2 size={12} className="animate-spin" /> : <><span>Continue</span><ChevronRight size={12} /></>}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

interface ChipInputProps {
  values: string[];
  onChange: (values: string[]) => void;
  placeholder?: string;
  singleLine?: boolean;
}

function ChipInput({ values, onChange, placeholder, singleLine }: ChipInputProps) {
  const [inputValue, setInputValue] = useState("");

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && inputValue.trim()) {
      e.preventDefault();
      if (!values.includes(inputValue.trim())) {
        onChange([...values, inputValue.trim()]);
      }
      setInputValue("");
    } else if (e.key === "Backspace" && !inputValue && values.length > 0) {
      onChange(values.slice(0, -1));
    }
  };

  return (
    <div
      className={`flex flex-wrap gap-1.5 p-2 bg-[var(--color-surface-2)] border border-[var(--color-border)] rounded-lg min-h-[40px] ${
        singleLine ? "items-center" : ""
      }`}
    >
      {values.map((val) => (
        <span
          key={val}
          className="inline-flex items-center gap-1 px-2 py-1 rounded bg-[var(--color-accent-bg)] border border-[var(--color-brand)] text-[12px] text-[var(--color-fg)]"
        >
          {val}
          <button
            onClick={() => onChange(values.filter((v) => v !== val))}
            className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]"
          >
            <X size={10} />
          </button>
        </span>
      ))}
      <input
        autoFocus
        value={inputValue}
        onChange={(e) => setInputValue(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder || "Type and press Enter..."}
        className={`flex-1 min-w-[100px] bg-transparent border-none outline-none text-[13px] text-[var(--color-fg)] placeholder:text-[var(--color-fg-subtle)] ${singleLine ? "py-0" : "py-1"}`}
      />
    </div>
  );
}