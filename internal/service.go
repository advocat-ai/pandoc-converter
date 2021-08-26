package internal

import (
	"context"
	"github.com/advocat-ai/pandoc-converter/api"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
)

var kFormats = map[api.Format]string {
	api.Format_ASCIIDOC: "asciidoc",
	api.Format_BEAMER: "beamer",
	api.Format_BIBTEX: "bibtex",
	api.Format_BIBLATEX: "biblatex",
	api.Format_COMMONMARK: "commonmark",
	api.Format_COMMONMARK_X: "commonmark_x",
	api.Format_CONTEXT: "context",
	api.Format_CSLJSON: "csljson",
	api.Format_DOCBOOK_4: "docbook4",
	api.Format_DOCBOOK_5: "docbook5",
	api.Format_DOCX: "docx",
	api.Format_DOKUWIKI: "dokuwiki",
	api.Format_EPUB_3: "epub3",
	api.Format_EPUB_2: "epub2",
	api.Format_FB2: "fb2",
	api.Format_GFM: "gfm",
	api.Format_HADDOCK: "haddock",
	api.Format_HTML_5: "html5",
	api.Format_HTML_4: "html4",
	api.Format_ICML: "icml",
	api.Format_IPYNB: "ipynb",
	api.Format_JATS_ARCHIVING: "jats_archiving",
	api.Format_JATS_ARTICLE_AUTHORING: "jats_articleauthoring",
	api.Format_JATS_PUBLISHING: "jats_publishing",
	api.Format_JIRA: "jira",
	api.Format_JSON: "json",
	api.Format_LATEX: "latex",
	api.Format_MAN: "man",
	api.Format_MARKDOWN: "markdown",
	api.Format_MARKDOWN_MMD: "markdown_mmd",
	api.Format_MARKDOWN_PHP_EXTRA: "markdown_phpextra",
	api.Format_MARKDOWN_STRICT: "markdown_strict",
	api.Format_MEDIAWIKI: "mediawiki",
	api.Format_MS: "ms",
	api.Format_MUSE: "muse",
	api.Format_NATIVE: "native",
	api.Format_ODT: "odt",
	api.Format_OPML: "opml",
	api.Format_OPENDOCUMENT: "opendocument",
	api.Format_ORG: "org",
	api.Format_PDF: "pdf",
	api.Format_PLAIN: "plain",
	api.Format_PPTX: "pptx",
	api.Format_RST: "rst",
	api.Format_RTF: "rtf",
	api.Format_TEXINFO: "texinfo",
	api.Format_TEXTILE: "textile",
	api.Format_SLIDEOUS: "slideous",
	api.Format_SLIDY: "slidy",
	api.Format_DZSLIDES: "dzslides",
	api.Format_REVEALJS: "revealjs",
	api.Format_S5: "s5",
	api.Format_TEI: "tei",
	api.Format_XWIKI: "xwiki",
	api.Format_ZIMWIKI: "zimwiki",
}

type ConverterService struct {
	api.ConverterServer
	pandoc string
	log *zap.Logger
	pathEnvVar string
}

type Opt func(*ConverterService)

func WithLog(log *zap.Logger) Opt {
	return func(s *ConverterService) {
		s.log = log
	}
}

func WithPandocPath(path string) Opt {
	return func (s *ConverterService) {
		s.pandoc = path
	}
}

func WithPathEnvVar(v string) Opt {
	return func (s *ConverterService) {
		s.pathEnvVar = v
	}
}

func NewConverterService(opts... Opt) (api.ConverterServer, error) {
	s := ConverterService{}

	for _, opt := range opts {
		opt(&s)
	}

	if s.pandoc == "" {
		pandoc, err := exec.LookPath("pandoc")
		if err != nil {
			return nil, err
		}
		s.pandoc = pandoc
	}

	if s.log == nil {
		log, err := zap.NewDevelopment()
		if err != nil {
			return nil, err
		}
		s.log = log.With(zap.String("pandoc", s.pandoc), zap.String("PATH", s.pathEnvVar))
	}

	return &s, nil
}

func (s *ConverterService) Convert(ctx context.Context, req	*api.ConvertRequest) (*api.ConvertResponse, error) {
	td, err := os.MkdirTemp(os.TempDir(), "session-*")
	if err != nil {
		s.log.Error("failed to create temporary directory", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to create temporary directory for call - %v", err)
	}

	log := s.log.With(zap.String("tempDir", td))
	log.Debug("created temporary directory")

	defer func() {
		log.Debug("removing temporary content and directory")
		err := os.RemoveAll(td)
		if err != nil {
			log.Warn("failed to remove temporary directory", zap.Error(err))
		} else {
			log.Debug("removal complete.")
		}
	}()

	inputFormat, found := kFormats[req.FromFormat]
	if !found {
		log.Error("invalid input format", zap.Int("fromFormat", int(req.FromFormat)))
		return nil, status.Errorf(codes.InvalidArgument, "invalid from format %v", req.FromFormat)
	}

	outputFormat, found := kFormats[req.ToFormat]
	if !found {
		log.Error("invalid output format", zap.Int("toFormat", int(req.ToFormat)))
		return nil, status.Errorf(codes.InvalidArgument, "invalid to format %v", req.ToFormat)
	}

	if len(req.Content) == 0 {
		log.Error("empty input content")
		return nil, status.Error(codes.InvalidArgument, "empty input content")
	}

	inputPath := path.Join(td, "input-file")
	outputPath := path.Join(td, "output-file")

	log = log.With(zap.String("inputFormat", inputFormat), zap.String("outputFormat", outputFormat), zap.String("inputPath", inputPath), zap.Int("inputSize", len(req.Content)), zap.String("outputPath", outputPath))

	log.Debug("writing input to temporary file")
	err = ioutil.WriteFile(inputPath, req.Content, fs.ModePerm)
	if err != nil {
		log.Error("failed to write input to temporary file", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to write input to temporary file - %v", err)
	}

	args := []string{
		s.pandoc,
	}

	args = append(args, "-i", inputPath, "-f", inputFormat, "-o", outputPath, "-t", outputFormat)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	log = log.With(zap.String("cmd", cmd.String()))

	log.Debug("calling pandoc")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("call failed", zap.ByteString("output", out), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "conversion failed - %v\n%v", err, out)
	}

	log.Debug("reading output file")
	outContent, err := ioutil.ReadFile(outputPath)
	if err != nil {
		log.Error("failed to read output file", zap.Error(err))
		return nil, status.Errorf(codes.Internal, "failed to read output file - %v", err)
	}

	log.Debug("call complete.")

	reply := api.ConvertResponse{
		ToFormat: req.ToFormat,
		Content: outContent,
	}

	return &reply, nil
}
