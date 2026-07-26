export function normalizeFavoriteSource(source) {
	const normalized = String(source ?? "").trim().toLowerCase();
	if (normalized === "") return "telegram";
	if (normalized === "telegram" || normalized === "youtube") return normalized;
	throw new Error("Favorite source must be telegram or youtube.");
}
