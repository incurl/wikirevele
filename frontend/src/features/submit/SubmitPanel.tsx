/**
 * The submit panel.
 *
 * Collects the prompt and the free-form parameters of a job. The
 * scaffold models no domain: the parameters are a plain key/value
 * list, which the runner receives as it is.
 */

import { useState } from 'react';

interface SubmitPanelProps {
  /** Disables the submit button while a job is being queued. */
  disabled: boolean;
  /** Called with the prompt and the parameters. */
  onSubmit: (prompt: string, params: Record<string, unknown>) => void;
}

/**
 * A single parameter of the job.
 */
interface ParamField {
  key: string;
  value: string;
}

/**
 * The submit panel.
 */
export function SubmitPanel({ disabled, onSubmit }: SubmitPanelProps) {
  const [prompt, setPrompt] = useState('');
  const [params, setParams] = useState<ParamField[]>([]);

  const canSubmit = prompt.trim() !== '' && !disabled;

  const addParam = () => {
    setParams((current) => [...current, { key: '', value: '' }]);
  };

  const updateParam = (index: number, field: 'key' | 'value', value: string) => {
    setParams((current) =>
      current.map((param, i) => (i === index ? { ...param, [field]: value } : param)),
    );
  };

  const removeParam = (index: number) => {
    setParams((current) => current.filter((_, i) => i !== index));
  };

  const submit = () => {
    if (!canSubmit) {
      return;
    }

    // The parameters with an empty key are dropped, so that a half
    // filled row does not end up in the job.
    const collected: Record<string, unknown> = {};
    for (const param of params) {
      const key = param.key.trim();
      if (key !== '') {
        collected[key] = param.value;
      }
    }

    onSubmit(prompt.trim(), collected);
    setPrompt('');
  };

  return (
    <section className="panel">
      <div className="panel-header">
        <span>New job</span>
      </div>
      <div className="panel-body">
        <div className="field">
          <label className="field-label" htmlFor="submit-prompt">
            Prompt
          </label>
          <textarea
            id="submit-prompt"
            rows={6}
            placeholder="What should the runner do?"
            value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
          />
        </div>

        <div className="field">
          <span className="field-label">
            Parameters
            <button type="button" className="btn btn-sm" onClick={addParam}>
              Add
            </button>
          </span>

          {params.length === 0 ? (
            <span className="text-dim">
              No parameters. The runner receives the prompt on its own.
            </span>
          ) : (
            params.map((param, index) => (
              <div className="param-row" key={index}>
                <input
                  type="text"
                  placeholder="name"
                  value={param.key}
                  onChange={(event) => updateParam(index, 'key', event.target.value)}
                />
                <input
                  type="text"
                  placeholder="value"
                  value={param.value}
                  onChange={(event) => updateParam(index, 'value', event.target.value)}
                />
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => removeParam(index)}
                  aria-label="Remove this parameter"
                >
                  ×
                </button>
              </div>
            ))
          )}
        </div>

        <button type="button" className="btn btn-primary" disabled={!canSubmit} onClick={submit}>
          Submit
        </button>
      </div>
    </section>
  );
}
