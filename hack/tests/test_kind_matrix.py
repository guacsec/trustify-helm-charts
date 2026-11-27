import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location(
    "kind_matrix", Path(__file__).resolve().parents[1] / "kind-matrix.py"
)
matrix = importlib.util.module_from_spec(spec)
spec.loader.exec_module(matrix)


class MatrixTest(unittest.TestCase):
    def test_semver_order_ignores_newer_backports_and_prereleases(self):
        releases = [{"tag_name": tag} for tag in [
            "v0.6.4", "v0.7.2", "v0.7.1", "v0.4.21", "v0.6.3", "nightly",
        ]] + [
            {"tag_name": "v0.8.0", "prerelease": True},
            {"tag_name": "v1.0.0", "draft": True},
        ]
        resolved = []

        def resolve(tag):
            resolved.append(tag)
            return f"image-for-{tag}"

        result = matrix.build_matrix(releases, resolve)["include"]
        self.assertEqual(resolved, ["latest", "0.7.z", "0.6.z"])
        self.assertEqual([(c["tag"], c["mode"]) for c in result], [
            ("latest", "psql"), ("0.7.z", "psql"), ("0.6.z", "psql"), ("latest", "trustd"),
        ])
        self.assertEqual(result[0]["image"], result[3]["image"])

    def test_future_major_and_numeric_minor_ordering(self):
        self.assertEqual(matrix.release_lines([
            {"tag_name": tag} for tag in ["v0.9.0", "v0.10.0", "v1.0.0"]
        ]), ["1.0.z", "0.10.z"])

    def test_missing_release_history_fails_instead_of_shrinking_matrix(self):
        with self.assertRaises(ValueError):
            matrix.release_lines([{"tag_name": "v0.7.2"}])


if __name__ == "__main__":
    unittest.main()
