#include <algorithm>
#include <cassert>
#include <iostream>
#include <numeric>
#include <stdexcept>
#include <vector>

namespace linalg {

template <typename T>
class Matrix {
public:
    Matrix(std::size_t rows, std::size_t cols, T fill = T{})
        : rows_(rows), cols_(cols), data_(rows * cols, fill) {}

    [[nodiscard]] std::size_t rows() const noexcept { return rows_; }
    [[nodiscard]] std::size_t cols() const noexcept { return cols_; }

    T& operator()(std::size_t r, std::size_t c) {
        assert(r < rows_ && c < cols_);
        return data_[r * cols_ + c];
    }

    const T& operator()(std::size_t r, std::size_t c) const {
        return const_cast<Matrix&>(*this)(r, c);
    }

    Matrix operator*(const Matrix& other) const {
        if (cols_ != other.rows_) {
            throw std::invalid_argument("dimension mismatch");
        }
        Matrix result(rows_, other.cols_);
        for (std::size_t i = 0; i < rows_; ++i) {
            for (std::size_t k = 0; k < cols_; ++k) {
                const T lhs = (*this)(i, k);
                for (std::size_t j = 0; j < other.cols_; ++j) {
                    result(i, j) += lhs * other(k, j);
                }
            }
        }
        return result;
    }

    T trace() const {
        T sum{};
        for (std::size_t i = 0; i < std::min(rows_, cols_); ++i) sum += (*this)(i, i);
        return sum;
    }

    friend std::ostream& operator<<(std::ostream& os, const Matrix& m) {
        for (std::size_t r = 0; r < m.rows_; ++r) {
            os << (r == 0 ? "[" : " ");
            for (std::size_t c = 0; c < m.cols_; ++c) os << m(r, c) << (c + 1 < m.cols_ ? ", " : "");
            os << (r + 1 == m.rows_ ? "]\n" : "\n");
        }
        return os;
    }

private:
    std::size_t rows_, cols_;
    std::vector<T> data_;
};

}  // namespace linalg

int main() {
    linalg::Matrix<double> a(2, 3, 1.5), b(3, 2, 2.0);
    auto c = a * b;
    std::cout << c << "trace = " << c.trace() << '\n';
    return 0;
}
