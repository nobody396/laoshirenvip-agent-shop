// Same decision tree as GMShop; explicit lsrai.shop mappings, never prices/name inference.
export const guideProducts = {
  "gpt500": {
    "productId": 31,
    "itemId": 35,
    "slug": "upstream-2-39921-1788865601112"
  },
  "gpt20ios": {
    "productId": 31,
    "itemId": 4,
    "slug": "upstream-2-39921-1788865601112"
  },
  "go": {
    "productId": 31,
    "itemId": 2,
    "slug": "upstream-2-39921-1788865601112"
  },
  "plusph": {
    "productId": 31,
    "itemId": 6,
    "slug": "upstream-2-39921-1788865601112"
  },
  "plusios": {
    "productId": 31,
    "itemId": 3,
    "slug": "upstream-2-39921-1788865601112"
  },
  "fiveios": {
    "productId": 31,
    "itemId": 5,
    "slug": "upstream-2-39921-1788865601112"
  },
  "fiveph": {
    "productId": 31,
    "itemId": 25,
    "slug": "upstream-2-39921-1788865601112"
  },
  "twentyrenew": {
    "productId": 35,
    "itemId": 29,
    "slug": "chatgpt-pro-20x-ph-renewal"
  },
  "twentynew": {
    "productId": 31,
    "itemId": 24,
    "slug": "upstream-2-39921-1788865601112"
  },
  "points250": {
    "productId": 19,
    "itemId": 8,
    "slug": "upstream-1-22-1788606583551"
  },
  "points500": {
    "productId": 19,
    "itemId": 9,
    "slug": "upstream-1-22-1788606583551"
  },
  "points1000": {
    "productId": 19,
    "itemId": 7,
    "slug": "upstream-1-22-1788606583551"
  },
  "points2500": {
    "productId": 19,
    "itemId": 30,
    "slug": "upstream-1-22-1788606583551"
  },
  "claudepro": {
    "productId": 21,
    "itemId": 11,
    "slug": "upstream-1-29-1788606602241"
  },
  "claudefive": {
    "productId": 21,
    "itemId": 10,
    "slug": "upstream-1-29-1788606602241"
  },
  "claudetwenty": {
    "productId": 21,
    "itemId": 27,
    "slug": "upstream-1-29-1788606602241"
  },
  "xpremium": {
    "productId": 24,
    "itemId": 14,
    "slug": "upstream-1-40-1788606604693"
  },
  "xplusmonth": {
    "productId": 23,
    "itemId": 13,
    "slug": "upstream-1-34-1788606603938"
  },
  "xplusyear": {
    "productId": 39,
    "itemId": 34,
    "slug": "x-premium-plus-annual"
  },
  "grok": {
    "productId": 22,
    "itemId": 12,
    "slug": "upstream-1-32-1788606603146"
  },
  "smsone": {
    "productId": 37,
    "itemId": 32,
    "slug": "codex-sms-one-time-us"
  },
  "smslong": {
    "productId": 38,
    "itemId": 33,
    "slug": "codex-sms-long-term-us"
  }
} as const;
export type GuideProduct = keyof typeof guideProducts;
export type GuideStep =
	| {
			kind: "question";
			id: string;
			options: string[];
			help?: "plan" | "billing" | "claude";
	  }
	| {
			kind: "result";
			product: GuideProduct;
			warning?: "overwrite" | "overwrite30" | "kyc";
	  }
	| {
			kind: "stop";
			reason:
				| "upgrade_pending"
				| "free_points"
				| "claude_active"
				| "rule_pending"
				| "fivehundred_active"
				| "wait_for_expiry"
				| "renew_bill_mismatch";
	  };
const question = (
	id: string,
	options: string[],
	help?: "plan" | "billing" | "claude",
): GuideStep => ({ kind: "question", id, options, help });
const result = (
	product: GuideProduct,
	warning?: "overwrite" | "overwrite30" | "kyc",
): GuideStep => ({ kind: "result", product, warning });

/** Small fixed decision tree. No state is inferred from an expiry date. */
export function guideStep(a: readonly string[]): GuideStep {
	const [family, current, target] = a;
	if (!family)
		return question("family", ["gpt", "claude", "points", "sms", "x", "grok"]);
	if (family === "gpt") {
		if (!current)
			return question(
				"gpt_current",
				["free", "go", "plus", "pro100", "pro200", "pro500"],
				"plan",
			);
		if (!["free", "go", "plus", "pro100", "pro200", "pro500"].includes(current))
			return { kind: "stop", reason: "rule_pending" };
		const hasPairedChannel = ["plus", "pro100", "pro200"].includes(current);
		const currentChannel = hasPairedChannel ? a[2] : undefined;
		if (hasPairedChannel && !currentChannel)
			return question(
				"current_channel",
				["current_ph", "current_ios", "current_other"],
				"billing",
			);
		if (
			hasPairedChannel &&
			currentChannel !== "current_ph" &&
			currentChannel !== "current_ios"
		)
			return { kind: "stop", reason: "rule_pending" };
		const [target, choice, detail, renewalChannel] = a.slice(
			hasPairedChannel ? 3 : 2,
		);
		if (!target)
			return question("target", [
				"go",
				"plus",
				"five",
				"twenty",
				"fivehundred",
			]);
		if (!["go", "plus", "five", "twenty", "fivehundred"].includes(target))
			return { kind: "stop", reason: "rule_pending" };
		if (current === "free") {
			if (target === "go") return result("go");
			if (target === "fivehundred") return result("gpt500");
			if (!choice) return question("channel", ["ph", "ios"]);
			if (choice !== "ph" && choice !== "ios")
				return { kind: "stop", reason: "rule_pending" };
			return result(
				target === "plus"
					? choice === "ph"
						? "plusph"
						: "plusios"
					: target === "five"
						? choice === "ph"
							? "fiveph"
							: "fiveios"
						: choice === "ph"
							? "twentynew"
							: "gpt20ios",
			);
		}
		if (!choice) return question("timing", ["recharge_now", "after_expiry"]);
		if (choice === "after_expiry")
			return {
				kind: "stop",
				reason:
					target === "fivehundred" ? "fivehundred_active" : "wait_for_expiry",
			};
		if (choice !== "recharge_now")
			return { kind: "stop", reason: "rule_pending" };
		if (target === "fivehundred")
			return { kind: "stop", reason: "fivehundred_active" };
		if (
			(target === "go" && current !== "go") ||
			(target === "plus" && current !== "go" && current !== "plus") ||
			(target === "five" && (current === "pro200" || current === "pro500")) ||
			(target === "twenty" && current === "pro500")
		)
			return { kind: "stop", reason: "rule_pending" };
		if (target === "go") return result("go", "overwrite30");
		if (
			current === "plus" &&
			currentChannel === "current_ph" &&
			target === "five"
		) {
			if (!detail) return question("upgrade_channel", ["ph_upgrade", "ios"]);
			if (detail === "ph_upgrade")
				return { kind: "stop", reason: "upgrade_pending" };
			if (detail !== "ios") return { kind: "stop", reason: "rule_pending" };
		}
		if (
			current === "pro200" &&
			currentChannel === "current_ph" &&
			target === "twenty"
		) {
			if (!detail)
				return question("pro_php", ["yes_8919", "no_8919"], "billing");
			if (detail !== "yes_8919")
				return { kind: "stop", reason: "renew_bill_mismatch" };
			if (!renewalChannel)
				return question("renew_channel", ["ph_renew", "ios"]);
			return renewalChannel === "ph_renew"
				? result("twentyrenew")
				: renewalChannel === "ios"
					? result("gpt20ios", "overwrite30")
					: { kind: "stop", reason: "rule_pending" };
		}
		return result(
			target === "plus"
				? "plusios"
				: target === "five"
					? "fiveios"
					: "gpt20ios",
			"overwrite30",
		);
	}
	if (family === "claude") {
		if (!current)
			return question(
				"claude_current",
				["claude_free", "claude_paid"],
				"claude",
			);
		if (current === "claude_paid")
			return { kind: "stop", reason: "claude_active" };
		if (!target)
			return question("claude_target", [
				"claudepro",
				"claudefive",
				"claudetwenty",
			]);
		if (
			target === "claudepro" ||
			target === "claudefive" ||
			target === "claudetwenty"
		)
			return result(target, target === "claudepro" ? undefined : "kyc");
	}
	if (family === "points") {
		if (!current)
			return question("current", ["free", "go", "plus", "pro"], "plan");
		if (current === "free") return { kind: "stop", reason: "free_points" };
		if (!target)
			return question("points_target", [
				"points250",
				"points500",
				"points1000",
				"points2500",
			]);
		if (
			target === "points250" ||
			target === "points500" ||
			target === "points1000" ||
			target === "points2500"
		)
			return result(target);
	}
	if (family === "sms") {
		if (!current) return question("sms_target", ["smsone", "smslong"]);
		if (current === "smsone" || current === "smslong") return result(current);
	}
	if (family === "x") {
		if (!current)
			return question("x_target", ["xpremium", "xplusmonth", "xplusyear"]);
		if (current === "xpremium" || current === "xplusmonth")
			return result(current, "overwrite");
		if (current === "xplusyear") return result(current);
	}
	if (family === "grok") return result("grok", "overwrite");
	return { kind: "stop", reason: "rule_pending" };
}

export function guideProductUrl(product: GuideProduct) {
  const ref = guideProducts[product];
  return `/products/${ref.slug}?sku=${ref.itemId}`;
}
