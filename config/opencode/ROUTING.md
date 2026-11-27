# Routing

## Precedence and accounts

- **The user's picks** of model, effort, or account override this page.
- **Model order:** an explicit `model`, then the `step` card's model.
- **Accounts:** write `claude/<model>#<variant>` so the plugin picks the account.
  - `anthropic`, `anthropic-personal`, and `anthropic-cullyn` pin one account.

## Models

Roughly ordered by the user's current trust and preference, which changes often.
A model without an effort note follows the defaults under Effort.

1. **Opus**, `claude/claude-opus-5-5`
   - The best default: the least likely to get things wrong, at a fair cost and speed.
   - It still writes sloppy work at times, so its builds need review.
   - It can be more thorough than appropriate, both a strength and a weakness.

2. **Fable**, `claude/claude-fable-5-2`
   - Stronger than Opus on the hardest decisions and broad, open-ended reviews.
   - It costs about twice as much per token, runs slower, and can be less thorough.
   - Its effort levels sit one step above Opus's.
     - Fable `high` roughly matches Opus `xhigh`, and `medium` matches `high`.
   - Fable `xhigh` is the strongest seat here; save it for the toughest calls.

3. **Neon**, `gemini/gemini-4-neon`
   - Unsettled: its reviews have found real bugs, but its prose can run wordy, maybe?
   - It does best at debug, simplify, and security review. Could it build well?
   - It runs through a bridge at this time that withholds two things:
     - It gets no agent prompt, so name its role and skill files in the brief.
     - It gets no delegation tools, so keep owner, entropy, and decide off `gemini/`.

4. **Sol**, `openai/gpt-6.2-sol`
   - **Strengths:** running tests and commands, web work, and review.
   - **Findings:** its reviews catch a lot but run noisy; verify ones only Sol raised.

5. **Astra**, `openai/gpt-6.2-astra`
   - **Strengths:** thorough as a checker or an extra seat on review councils.
   - **Weakness:** it can miss the main point of a problem.
     - It decides critical calls less reliably than Fable.

6. **Sonnet**, `claude/claude-sonnet-5-5`
   - **Strengths:** speed on context maps, dirty-tree reads, and bounded jobs.
   - **Effort:** avoid `medium`.

7. **Hydrogen**, `gemini/gemini-4-hydrogen`
   - Fast but shallow in review, with no good read on it yet beyond simple scouts.
   - It could be better, but could be slower.
   - Use it on council scouts, so comparisons can be collected.

8. **DeepSeek**, `opencode-go/deepseek-v4.1-flash`
   - **Strengths:** web research that finds facts other seats miss, at cents a seat.

9. **GLM 5.3**, `opencode-go/glm-5.3`
   - **Weaknesses:** it leans on vendor docs in web research, and builds poorly.
   - **Effort:** pass `#high`, since it defaults to `max`.

10. **Grok 4.7**, `xai/grok-4.7` and `opencode-go/grok-4.7`
    - **`x` tool:** it runs X search; when xAI is capped, tell web scouts to skip it.
    - **Cost:** one scout on Go spends about 15% of its weekly cap.

## Effort

Pick effort per task from speed, stakes, and usage headroom.

- **`medium`**: fully specified work when speed and cost matter.
  - It misses things and hallucinates more easily.
- **`high`**: the default; it makes fewer mistakes than `medium`.
  - It can miss things at this level, so step up when that matters.
- **`xhigh`**: often the better choice, on any agent, scouts included.
  - Use it freely when the week has usage to spare, such as near a weekly reset.
  - A bug resisting fixes also calls for it.
  - It can slow things down, so be wary when more iteration is expected.
- **`max`**: generally only when the user asks.
  - Long unattended workflows with plenty of usage may tastefully weave in a few.

## Seats and councils

One strong seat at `xhigh` can cover several concerns with one read and one report.
Separate seats pay off when models catch different things or a miss costs a lot.

- **One seat** suits small fixes, concerns that share code, and tight quotas.
- **More seats** suit security, production, and design verdicts, and web research.
- **Lenses:** split seats by risk, or repeat one lens across models to compare them.
- **Duplicates:** two seats with one model and one lens mostly repeat each other.
- **Sol and Astra seats** spend weekly OpenAI credits that otherwise go unused.

**Why run a council:**

A council gives models from different providers the same brief, blind to each other.

- **Diversity:** one model reinforces its own blind spots; another breaks the rut.
- **Spare usage:** with quota and time to spare, an extra seat costs little; run it.
- **Comparison:** each council shows which model did better on that kind of work.
  - Trust in a model is how rarely it gets things wrong, weighed with cost and taste.
  - Design and copy have no settled pick yet, so council them when usage allows.

**Keep the record comparable:**

- Give every seat the same brief and baseline.
- In the synthesis, name the seat behind each finding and say whether it held.
  - A later sweep can then score each model without anyone logging by hand.

**What each model catches:**

- **Security:** Neon and Sol each found leaks the other missed.
- **Review:** Neon has found real bugs that Sol missed.
- **Opus** filters Sol and Astra findings well, as a critic or a verdict seat.
  - Its verdicts add the most on harness, production, and design.
- **Web:** DeepSeek, Sol, Opus, and Sonnet each found sources the others missed.
- **Tendril** is a Grok Bot research seat on the `x` pool, not a `provider/model`.
  - Collab drives it through the `grok-bot` skill, so `subagent` cannot launch it.
  - In a 2026-11-27 bake-off, a blind judge (Sol) scored it 4.7 of 5 on average, best on 2 of 3 questions.
    - A DeepSeek web scout with working search scored 3.7 and won the third; scouts without working search scored 3.0.
    - That was one judge and three questions, and Opus never reran with working search.
  - **Cost:** about 0.7% of the X weekly Grok Bot pool per question, roughly 140 a week from X Premium+, with no extra spend.
  - A question takes 5–10 minutes, and one bot takes one question at a time.

## Refusals

A refusal arrives as a provider error that ends the turn, not as prose.
Each provider trips on different things, so the fix is usually another provider.

- **OpenAI** refuses security reviews worded as attacks: hunt, bypass, exploit.
  - Rewording has not worked on Sol; rerun the seat on Neon.
  - A cyber error that the harness retries mid-stream needs no action.
- **Anthropic** blocks any agent whose context holds exploit or bypass detail.
  - That has hit builders, the decider, and root sessions, not only reviews.
  - Rerun on Sol or Astra.
- **Gemini** has not refused yet, so Neon is the fallback for security review.

## Usage

- **Caps are facts to weigh** against the task's stakes and the reset time.
- Check `usage_status` before a launch outside `claude` and `cursor`.
- **Trend** gets no new jobs near the top of its 5-hour window; the plugin enforces it.
- **Cursor:** its small Opus and Sonnet pool suits short, bounded jobs.

## Plugin automation

- **On its own,** the lanes plugin picks the Anthropic account for `claude/` models.
  - It moves a capped Claude model to the same model on Cursor.
  - Its launch notes show the pool's headroom and reset time.
- **The caller** chooses the model and effort.
  - A launch without a model errors and points to this page.
  - A capped model errors and lists the open options, so the caller picks again.
