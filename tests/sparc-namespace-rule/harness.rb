# Runs sparc's SparcNamespacePropsRule, read-only, over a folder of OSCAL.
#   ruby tests/sparc-namespace-rule/harness.rb <sparc checkout> <folder>
# See README.md beside this file (#122).
require "json"
require "pathname"
sparc = Pathname.new(ARGV.fetch(0)); dir = ARGV.fetch(1)
module Rails; def self.root = SPARC_ROOT; end
SPARC_ROOT = sparc
class OscalValidationError < StandardError; end
module SparcNamespaceProps
  NS = "https://sparc.risk-sentinel.org/ns"
  NAMES = %w[node-type parent-uuid next-decision-date fips-199 blocks-ato evidence-kind signed-by condition-expires trigger].freeze
  # Verbatim from sparc app/services/sparc_namespace_props.rb.
  def self.ours?(prop, names = NAMES) = prop.is_a?(Hash) && prop["ns"] == NS && names.include?(prop["name"])
end
load sparc.join("app/services/sparc_namespace_props_rule.rb").to_s
models = { "system-security-plan" => :ssp, "plan-of-action-and-milestones" => :poam, "assessment-results" => :assessment_results }
checked = 0; failed = 0; props = 0
Dir[File.join(dir, "*.json")].sort.each do |f|
  data = JSON.parse(File.read(f)) rescue next
  next unless data.is_a?(Hash)
  key = (data.keys & models.keys).first or next
  props += JSON.generate(data).scan('"ns":"https://sparc.risk-sentinel.org/ns"').size
  errs = SparcNamespacePropsRule.new(models[key], data).errors
  checked += 1
  next if errs.empty?
  failed += 1
  puts "FAIL #{File.basename(f)}"; errs.each { |e| puts "  #{e}" }
end
puts "#{checked} documents checked, #{failed} failed, #{props} SPARC-namespace props seen"
exit(failed.zero? && checked.positive? ? 0 : 1)
