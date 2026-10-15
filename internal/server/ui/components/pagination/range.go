package pagination

func CreatePagination(currentPage, totalPages, maxVisible int) struct {
	CurrentPage int
	TotalPages  int
	Pages       []int
	HasPrevious bool
	HasNext     bool
} {
	if currentPage < 1 {
		currentPage = 1
	}
	if totalPages < 1 {
		totalPages = 1
	}
	if currentPage > totalPages {
		currentPage = totalPages
	}
	if maxVisible < 1 {
		maxVisible = 5
	}

	start, end := visibleRange(currentPage, totalPages, maxVisible)
	pages := make([]int, 0, end-start+1)
	for i := start; i <= end; i++ {
		pages = append(pages, i)
	}

	return struct {
		CurrentPage int
		TotalPages  int
		Pages       []int
		HasPrevious bool
		HasNext     bool
	}{
		CurrentPage: currentPage,
		TotalPages:  totalPages,
		Pages:       pages,
		HasPrevious: currentPage > 1,
		HasNext:     currentPage < totalPages,
	}
}

func visibleRange(currentPage, totalPages, maxVisible int) (int, int) {
	if totalPages <= maxVisible {
		return 1, totalPages
	}

	half := maxVisible / 2
	start := currentPage - half
	end := currentPage + half

	if start < 1 {
		end += 1 - start
		start = 1
	}
	if end > totalPages {
		start -= end - totalPages
		if start < 1 {
			start = 1
		}
		end = totalPages
	}
	return start, end
}
